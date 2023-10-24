package blocks

import (
	"fmt"
	"log"

	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
	"github.com/freehandle/papirus"
)

type BlockListenerConfig struct {
	NodeAddr    string
	NodeToken   crypto.Token
	Credentials crypto.PrivateKey
}

type BlockIndex struct {
	storagecount int
	offset       int64
}

type BlockStore struct {
	Storage  []papirus.ByteStore
	Blocks   []BlockIndex
	Current  *chain.BlockBuilder
	Unselaed []*chain.BlockBuilder
	Sealed   []*chain.SealedBlock
}

func (b *BlockStore) GetBlock(epoch int) []byte {
	if epoch >= len(b.Blocks) {
		return nil
	}
	index := b.Blocks[epoch]
	store := b.Storage[index.storagecount]
	data := store.ReadAt(index.offset, 4)
	size, _ := util.ParseUint32(data, 0)
	data = store.ReadAt(index.offset+4, int64(size))
	return data
}

func (b *BlockStore) New(header chain.BlockHeader) {
	if b.Current != nil {
		b.Unselaed = append(b.Unselaed, b.Current)
	}
	b.Current = &chain.BlockBuilder{
		Header:  header,
		Actions: chain.NewActionArray(),
	}
}

func (b *BlockStore) Action(action []byte) {
	b.Current.Actions.Append(action)
}

func prependSize(data []byte) []byte {
	bytes := make([]byte, 0)
	util.PutUint32(uint32(len(data)), &bytes)
	return append(bytes, data...)
}

func (b *BlockStore) Seal(epoch uint64, seal chain.BlockSeal) {
	var block *chain.BlockBuilder
	if b.Current.Header.Epoch == epoch {
		block = b.Current
		b.Current = nil
	} else {
		for n, recent := range b.Unselaed {
			if recent.Header.Epoch == epoch {
				block = recent
				b.Unselaed = append(b.Unselaed[0:n], b.Unselaed[n+1:]...)
				break
			}
		}
	}
	if block == nil {
		log.Printf("could not find block for seal %d", epoch)
		return
	}
	sealed := chain.SealedBlock{
		Header:  block.Header,
		Actions: block.Actions,
		Seal:    seal,
	}
	b.Sealed = append(b.Sealed, &sealed)
}

func (b *BlockStore) AppendBlock(commit *chain.CommitBlock) {
	bytes := commit.Serialize()
	storage := b.Storage[len(b.Storage)-1]
	index := BlockIndex{storagecount: len(b.Storage) - 1, offset: storage.Size()}
	b.Blocks = append(b.Blocks, index)
	storage.Append(prependSize(bytes))
	if commit.Actions.Len() > 0 {
		fmt.Printf("block %v: %v actions\n", commit.Header.Epoch, commit.Actions.Len())
	}
}

func (b *BlockStore) Commit(epoch uint64, commit chain.BlockCommit) {
	var block *chain.SealedBlock
	for n, recent := range b.Sealed {
		if recent.Header.Epoch == epoch {
			b.Sealed = append(b.Sealed[0:n], b.Sealed[n+1:]...)
			block = recent
			break
		}
	}
	if block == nil {
		log.Printf("could not find block for commit %d", epoch)
		return
	}
	commited := chain.CommitBlock{
		Header:  block.Header,
		Actions: block.Actions,
		Seal:    block.Seal,
		Commit:  &commit,
	}
	b.AppendBlock(&commited)
}

func NewBlockListener(config BlockListenerConfig, storage *BlockStore) chan error {

	finalize := make(chan error, 2)

	conn, err := socket.Dial(config.NodeAddr, config.Credentials, config.NodeToken)
	if err != nil {
		finalize <- err
		return finalize
	}
	conn.Send([]byte{chain.MsgSyncRequest, 1, 0, 0, 0, 0, 0, 0, 0})
	go func() {
		for {
			data, err := conn.Read()
			if err != nil || len(data) == 0 {
				fmt.Println(err)
				finalize <- err
				return
			}
			switch data[0] {
			case chain.MsgAction:
				storage.Action(data[1:])
			case chain.MsgNewBlock:
				header := chain.ParseBlockHeader(data[1:])
				if header != nil {
					storage.New(*header)
				} else {
					log.Print("could not parse header from node")
				}
			case chain.MsgSealBlock:
				ok := false
				if len(data) > 9 {
					epoch, _ := util.ParseUint64(data, 1)
					seal := chain.ParseBlockSeal(data[9:])
					if seal != nil {
						ok = true
						storage.Seal(epoch, *seal)
					}
				}
				if !ok {
					log.Print("could not parse block seal from node")
				}
			case chain.MsgCommitBlock:
				ok := false
				if len(data) > 9 {
					epoch, _ := util.ParseUint64(data, 1)
					commit := chain.ParseBlockCommit(data[9:])
					if commit != nil {
						ok = true
						storage.Commit(epoch, *commit)
					}
				}
				if !ok {
					log.Print("could not parse block seal from node")
				}
			case chain.MsgBlockCommitted:
				block := chain.ParseCommitBlock(data[1:])
				if block != nil {
					storage.AppendBlock(block)
				} else {
					log.Print("could not parse committed block from node")
				}
			case chain.MsgBlockSealed:
				block := chain.ParseSealedBlock(data[1:])
				if block != nil {
					storage.Sealed = append(storage.Sealed, block)
				} else {
					log.Print("could not parse sealed block from node")
				}

			}
		}
	}()
	return finalize
}
