package blocks

import (
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/protocol/actions"
	"github.com/freehandle/breeze/util"
)

func (b *BlockStore) FilterProtocolOnBlock(epoch uint64, output chain.ActionArray, filter func(uint32) bool) {
	block := b.GetBlock(int(epoch))
	all, _ := util.ParseActionsArray(block, chain.BlockActionOffset)
	for _, action := range all {
		if filter(actions.Protocol(action)) {
			output.Append(action)
		}
	}
}

func (b *BlockStore) FilterProtocolAllBlocks(output chain.ActionArray, filter func(uint32) bool) {
	blockCount := len(b.Blocks) // snapshot of all blocks at the instant
	for epoch := 0; epoch < blockCount; epoch++ {
		b.FilterProtocolOnBlock(uint64(epoch), output, filter)
	}
}
