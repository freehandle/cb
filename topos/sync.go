package topos

import (
	"errors"
	"fmt"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/cb/social"
)

type SyncSocialConfig struct {
	ProviderAddress string
	ProviderToken   crypto.Token
	NodeAddress     string
	NodeToken       crypto.Token
	Credentials     crypto.PrivateKey
	BlockStore      Chainer
}

type Chainer interface {
	Epoch() uint64
	AddBlock(block *social.ProtocolBlock)
}

type BlockSorter struct {
	incomplete []*social.ProtocolBlock
	last       uint64
	sequence   chan *social.ProtocolBlock
}

func NewBlockSorter(epoch uint64, send chan *social.ProtocolBlock) *BlockSorter {
	return &BlockSorter{
		incomplete: make([]*social.ProtocolBlock, 0),
		last:       epoch,
		sequence:   send,
	}
}

func (b *BlockSorter) Epoch() uint64 {
	return b.last
}

func (b *BlockSorter) AddBlock(block *social.ProtocolBlock) {
	if block == nil {
		b.sequence <- nil
		return
	}
	if block.Epoch <= b.last {
		return
	}
	if block.Epoch == b.last+1 {
		b.sequence <- block
		return
	}
	for n, incomplete := range b.incomplete {
		if block.Epoch == incomplete.Epoch {
			return
		}
		if incomplete.Epoch < block.Epoch {
			b.incomplete = append(b.incomplete[:n], append([]*social.ProtocolBlock{block}, b.incomplete[n:]...)...)
			return
		}
	}
	b.incomplete = append(b.incomplete, block)
}

func SyncSocial(config SyncSocialConfig) chan error {
	finalize := make(chan error, 2)
	startSyncEpoch := config.BlockStore.Epoch() + 1
	oldBlocks, lastEpoch, err := ReceiveBlocks(config.ProviderAddress, config.ProviderToken, config.Credentials, startSyncEpoch)
	if err != nil {
		finalize <- fmt.Errorf("could not receive blocks: %v", err)
		return finalize
	}
	newBlocks := social.SocialProtocolBlockListener(config.ProviderAddress, config.ProviderToken, config.Credentials, lastEpoch)
	go func() {
		live := true
		finishedOld := false
		for {
			select {
			case block := <-oldBlocks:
				if block != nil {
					config.BlockStore.AddBlock(block)
				} else {
					finishedOld = true
					if !live {
						return
					}
				}
			case block := <-newBlocks:
				if block == nil {
					live = false
					if finishedOld {
						finalize <- errors.New("connection to provider interrupted")
						return
					} else {
						config.BlockStore.AddBlock(block)
					}
				}
			}
		}
	}()
	return finalize
}
