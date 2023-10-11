package topos

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
)

type Blockchain struct {
	mu      sync.Mutex
	file    *os.File
	blocks  []int64
	current *MemoryBlock
	state   ProtocolState
	strict  bool
}

func (b *Blockchain) UpdateState(state ProtocolState) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = state
	return UpdateStateWithChain(state, b, b.strict)
}

// sync a new connection
func (b *Blockchain) Sync(conn *socket.CachedConnection, epoch uint64) {
	b.mu.Lock()
	currentCache := b.current.Clone()
	b.mu.Unlock()
	for n := epoch; n <= currentCache.epoch; n++ {
		var block *MemoryBlock
		var hash crypto.Hash
		if n == currentCache.epoch {
			block = currentCache
			hash = crypto.ZeroHash
		} else {
			var err error
			block, err = b.Block(n)
			if err != nil {
				conn.Close()
				return
			}
			hash = block.Hash()
		}
		data := []byte{MsgBlock}
		util.PutUint64(n, &data)
		util.PutHash(hash, &data)
		conn.SendDirect(data)
		for l := 0; l < block.Len(); l++ {
			action := block.Get(l)
			conn.SendDirect(append([]byte{MsgAction}, action...))
		}
	}
	conn.Ready()
}

func (b *Blockchain) Close() error {
	return b.file.Close()
}

func (b *Blockchain) NextBlock(epoch uint64, hash crypto.Hash) error {
	if (!hash.Equal(b.current.Hash())) && b.strict {
		return errors.New("block hash mismatch in strict mode")
	}
	if b.state != nil {
		if err := b.state.NextBlock(epoch); err != nil {
			return fmt.Errorf("block update incompatible with state update: %v", err)
		}
	}
	b.file.Seek(0, 2)
	data := []byte{MsgBlock}
	util.PutUint64(b.current.epoch, &data)
	if n, err := b.file.Write(data); n != len(data) {
		return fmt.Errorf("could not write to blockchain file: %v", err)
	}
	for n := 0; n < b.current.Len(); n++ {
		bytes := b.current.Get(n)
		data := []byte{MsgAction}
		util.PutUint64(uint64(len(bytes)), &data)
		data = append(data, bytes...)
		if n, err := b.file.Write(data); n != len(data) {
			return fmt.Errorf("could not write to blockchain file: %v", err)
		}
	}
	b.current = &MemoryBlock{
		epoch:   b.current.epoch + 1,
		data:    make([]byte, 0),
		actions: make([]int, 0),
	}
	return nil
}

func (b *Blockchain) Retrieve(positions map[uint64][]int) [][]byte {
	epochs := make(sort.IntSlice, 0, len(positions))
	for epoch, _ := range positions {
		epochs = append(epochs, int(epoch))
	}
	epochs.Sort()
	output := make([][]byte, 0)
	for _, epoch := range epochs {
		seq := positions[uint64(epoch)]
		data := b.RetrieveEpoch(uint64(epoch), seq)
		if len(data) > 0 {
			output = append(output, data...)
		}
	}
	return output
}

func (b *Blockchain) RetrieveEpoch(height uint64, sequences []int) [][]byte {
	var block *MemoryBlock
	if height == b.current.epoch {
		block = b.current
	} else if height < b.current.epoch {
		var err error
		if block, err = b.Block(height); err != nil {
			return nil
		}
	}
	if block == nil {
		return nil
	}
	output := make([][]byte, 0)
	for _, seq := range sequences {
		if seq >= b.current.Len() {
			return nil
		}
		output = append(output, b.current.Get(seq))
	}
	return output
}

func (b *Blockchain) Block(height uint64) (*MemoryBlock, error) {
	if int(height) >= len(b.blocks) {
		return nil, errors.New("height out of range")
	}
	pos := b.blocks[int(height)]
	block := MemoryBlock{
		epoch:   height,
		data:    make([]byte, 0),
		actions: make([]int, 0),
	}
	msg := make([]byte, 9)
	for {
		if n, err := b.file.ReadAt(msg, pos); n != 9 {
			if err == io.EOF {
				return &block, nil
			}
			return nil, fmt.Errorf("could not parse blockchain file: %v", err)
		}
		value, _ := util.ParseUint64(msg, 1)
		if msg[0] == MsgBlock {
			return &block, nil
		} else if msg[1] == MsgAction {
			pos = pos + 9
			action := make([]byte, int(value))
			if n, err := b.file.ReadAt(action, pos); n != len(action) {
				return nil, fmt.Errorf("could not parse blockchain file: %v", err)
			}
			block.Append(action) // nil = no broadcast
		} else {
			return nil, fmt.Errorf("invalid message type on file at position %v", pos)
		}
	}
}

func (b *Blockchain) Append(data []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var err error
	if b.state != nil {
		err = b.state.Action(data)
	}
	if err != nil && b.strict {
		return err
	}
	b.current.Append(data)
	return err
}

func OpenFSBlockchain(path string, strict bool) (*Blockchain, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	pos := int64(0)
	msg := make([]byte, 9)
	height := uint64(0)
	chain := Blockchain{
		mu:     sync.Mutex{},
		file:   file,
		blocks: make([]int64, 0),
		strict: strict,
	}
	for {
		if n, err := file.ReadAt(msg, pos); n != 9 {
			if err == io.EOF {
				return &chain, nil
			}
			return nil, fmt.Errorf("could not parse blockchain file: %v", err)
		}
		value, _ := util.ParseUint64(msg, 1)
		if msg[0] == MsgBlock {
			if value > 0 && value != height+1 {
				return nil, fmt.Errorf("block out of sequence on file at position %v", pos)
			}
			height = value
			pos = pos + 9
			chain.blocks = append(chain.blocks, pos)
		} else if msg[0] == MsgAction {
			pos = pos + 9 + int64(value)
		} else {
			return nil, fmt.Errorf("invalid message type on file at position %v", pos)
		}
	}
}

type RecentBlocks struct {
	mu        sync.Mutex
	blocks    []*MemoryBlock
	current   *MemoryBlock
	maxBlocks int
}

func (r *RecentBlocks) Sync(conn *socket.CachedConnection, epoch uint64) {
	r.mu.Lock()
	shift := int(epoch) - int(r.blocks[0].epoch)
	cache := make([]*MemoryBlock, 0)
	if shift > 1 {
		for n := shift; n < len(r.blocks); n++ {
			cache = append(cache, r.blocks[n])
		}
		cache = append(cache, r.current.Clone())
	}
	r.mu.Unlock()
	if shift < 1 {
		conn.Send(append([]byte{MsgSyncError}, []byte("node does not have information that old")...))
		conn.Close()
		conn.Live = false
		return
	}
	for n := 1; n < len(cache); n++ {
		hash := cache[n-1].Hash()
		epoch := cache[n].epoch
		data := []byte{MsgBlock}
		util.PutUint64(epoch, &data)
		util.PutHash(hash, &data)
		conn.SendDirect(data)
		for l := 0; l < cache[n].Len(); l++ {
			action := cache[n].Get(l)
			conn.SendDirect(append([]byte{MsgAction}, action...))
		}
	}
	conn.Ready()
}

func NewRecentBlocks(maxBlocks int, epoch uint64) *RecentBlocks {
	return &RecentBlocks{
		mu:     sync.Mutex{},
		blocks: make([]*MemoryBlock, 0),
		current: &MemoryBlock{
			epoch:   epoch,
			actions: make([]int, 0),
			data:    make([]byte, 0),
		},
		maxBlocks: maxBlocks,
	}
}

func (r *RecentBlocks) Append(action []byte) {
	r.current.Append(action)
}

func (r *RecentBlocks) NextBlock() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blocks = append(r.blocks, r.current)
	if len(r.blocks) > r.maxBlocks {
		r.blocks = r.blocks[1:]
	}
	r.current = &MemoryBlock{
		epoch:   r.current.epoch + 1,
		actions: make([]int, 0),
		data:    make([]byte, 0),
	}
}

func (r *RecentBlocks) Clone(epoch uint64) []*MemoryBlock {
	cloned := make([]*MemoryBlock, 0)
	for n := 0; n < len(r.blocks); n++ {
		if r.blocks[n].epoch >= epoch {
			cloned = append(cloned, r.blocks[n:]...)
			return cloned
		}
	}
	return cloned
}
