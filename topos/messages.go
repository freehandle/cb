package topos

import (
	"fmt"

	"github.com/freehandle/breeze/util"
)

const (
	MsgBlock byte = iota
	MsgAction
	MsgSyncRequest
	MsgActionSubmit
	MsgSyncError
)

type ProtocolState interface {
	Action(data []byte) error
	NextBlock(uint64) error
	Epoch() uint64
}

func UpdateStateWithChain(state ProtocolState, chain *Blockchain, strict bool) error {
	chain.mu.Lock()
	defer chain.mu.Unlock()
	start := state.Epoch()
	end := chain.current.epoch
	if start > end {
		return fmt.Errorf("cannot update state with chain: state epoch %v is greater than chain epoch %v", start, end)
	}
	if start == end {
		return nil
	}
	for n := start; n <= end; n++ {
		var block *MemoryBlock
		if n == end {
			block = chain.current
		} else {
			if b, err := chain.Block(n); err != nil {
				return fmt.Errorf("could not read block %v: %v", n, err)
			} else {
				block = b
			}
		}
		for l := 0; l < block.Len(); l++ {
			if err := state.Action(block.Get(l)); err != nil && strict {
				return fmt.Errorf("invalid action in strict mode: action %v in block %v: %v", l, n, err)
			}
		}
	}
	return nil
}

func NewSyncRequest(epoch uint64) []byte {
	data := []byte{MsgSyncRequest}
	util.PutUint64(epoch, &data)
	return data
}
