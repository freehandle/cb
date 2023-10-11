package blocks

import (
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/protocol/actions"
	"github.com/freehandle/breeze/util"
)

type QueryBlock struct {
	Epoch   uint64
	Actions []int // must be sorted
}

type Sender interface {
	Send([]byte) error
}

type Query struct {
	Data           []*QueryBlock
	FilterProtocol bool
	FilterFunc     func(uint32) bool
}

func (b *BlockStore) ExecuteQuery(query *Query, response Sender) chan error {
	finalize := make(chan error)
	go func() {
		for _, queryBlock := range query.Data {
			block := b.GetBlock(int(queryBlock.Epoch))
			count, offset := util.ParseUint32(block, chain.BlockActionOffset)
			nextAction := queryBlock.Actions[0]
			actionsProcessed := 0
			for n := 0; n < int(count); n++ {
				size := int(block[offset]) | int(block[offset+1])<<8
				if n == nextAction {
					candidate := block[offset+2 : offset+2+int(size)]
					if query.FilterProtocol && query.FilterFunc(actions.Protocol(candidate)) {
						if err := response.Send(block[offset+2 : offset+2+int(size)]); err != nil {
							finalize <- err
							return
						}
					}
					actionsProcessed += 1
					if actionsProcessed == len(queryBlock.Actions) {
						break
					}
					nextAction = queryBlock.Actions[actionsProcessed]
				}
				offset = offset + 2 + size
			}
		}
		finalize <- nil
	}()
	return finalize
}
