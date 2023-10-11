package social

import (
	"fmt"
	"log"
	"sync"

	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
)

const (
	StatusLive byte = iota
	StatusDone
	StatusSealed
	StatusCommit
)

type ActionBlock[M Merger[M]] struct {
	Epoch       uint64
	Checkpoint  uint64
	Origin      crypto.Hash
	Actions     *chain.ActionArray // validated in the build phase against checkpoint
	Invalidated []crypto.Hash      // invalidated in the commit phase
	Status      byte
	mutations   M
}

func (a *ActionBlock[M]) Clone() *ActionBlock[M] {
	return &ActionBlock[M]{
		Epoch:       a.Epoch,
		Checkpoint:  a.Checkpoint,
		Origin:      a.Origin,
		Actions:     a.Actions.Clone(),
		Invalidated: a.Invalidated,
		Status:      a.Status,
	}
}

type UniversalChain[M Merger[M], B Blocker[M]] struct {
	mu              sync.Mutex
	epoch           uint64 //live epoch
	validator       B      // current block for new actions
	liveBlock       *ActionBlock[M]
	commitState     Stateful[M, B]
	stateEpoch      uint64
	lastCommitEpoch uint64            // lastCommitEpoch can be different to stateEpoch in clone mode
	blocks          []*ActionBlock[M] // all blocks
	recentBlocks    []*ActionBlock[M]
	Transform       func([]byte) []byte
	checksumEpoch   uint64
}

func NewUniversalChain[M Merger[M], B Blocker[M]](state Stateful[M, B], epoch uint64) *UniversalChain[M, B] {
	return &UniversalChain[M, B]{
		mu:              sync.Mutex{},
		epoch:           epoch,
		validator:       state.Validator(),
		commitState:     state,
		lastCommitEpoch: epoch,
		blocks:          make([]*ActionBlock[M], 0),
		recentBlocks:    make([]*ActionBlock[M], 0),
	}
}

func (c *UniversalChain[M, B]) Validate(action []byte) bool {
	if c.Transform != nil {
		action = c.Transform(action)
	}
	if len(action) > 0 {
		return c.validator.Validate(action)
	}
	return false
}

func (c *UniversalChain[M, B]) SealBlock(epoch uint64) (crypto.Hash, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, block := range c.blocks {
		if block.Epoch == epoch {
			if block.Status == StatusLive || block.Status == StatusDone {
				block.Status = StatusSealed
				if block.Actions == nil {
					return crypto.ZeroValueHash, nil
				}
				return block.Actions.Hash(), nil
			}
			return crypto.ZeroHash, fmt.Errorf("SealBlock: block %v is not live or done", epoch)
		}
	}

	return crypto.ZeroHash, fmt.Errorf("SealBlock: block %v not found", epoch)
}

func (c *UniversalChain[M, B]) CommitBlock(epoch uint64, invalidated []crypto.Hash) ([]crypto.Hash, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, block := range c.blocks {
		if block.Epoch == epoch {
			if block.Status == StatusSealed {
				block.Status = StatusCommit
				return block.Invalidated, nil
			}
			return nil, fmt.Errorf("CommitBlock: block %v is not sealed", epoch)
		}
	}
	return nil, fmt.Errorf("CommitBlock: block %v not found", epoch)
}

func (c *UniversalChain[M, B]) Incorporate(epoch uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if epoch != c.stateEpoch+1 {
		return fmt.Errorf("Incorporate non-sequential block: block epoch %v vs state epoch %v", epoch, c.stateEpoch)
	}
	if c.liveBlock.Epoch == epoch {
		fmt.Errorf("Incorporate: block %v is not finalized", epoch)
	}
	for _, block := range c.blocks {
		if block.Epoch == epoch {
			if block.Status == StatusCommit {
				c.commitState.Incorporate(block.mutations)
				return nil
			}
			return fmt.Errorf("Incorporate: block %v is not committed", epoch)
		}
	}
	return fmt.Errorf("could not find block %v", epoch)
}

func (c *UniversalChain[M, B]) Rollback(epoch uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if epoch <= c.stateEpoch {
		return fmt.Errorf("Rollback request to and epoch before state epoch: rollback to %v vs state epoch %v", epoch, c.stateEpoch)
	}
	lastCommit := c.stateEpoch
	for n, block := range c.blocks {
		if block.Status == StatusCommit {
			lastCommit = block.Epoch
		}
		if block.Epoch == epoch {
			c.blocks = c.blocks[:n]
			c.lastCommitEpoch = lastCommit
			return nil
		}
	}
	return fmt.Errorf("Rollback: could not find block %v", epoch)
}

func (c *UniversalChain[M, B]) NewBlock() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch += 1
	mutations := make([]M, 0)
	for _, block := range c.blocks {
		if block.Epoch <= c.lastCommitEpoch {
			if block.Status == StatusCommit {
				mutations = append(mutations, block.mutations)
			} else {
				log.Printf("found non-commit %v before lastcommitEpoch %v", block.Epoch, c.lastCommitEpoch)
				return 0
			}
		}
		if block.Epoch == c.lastCommitEpoch {
			c.validator = c.commitState.Validator(mutations...)
			return c.epoch
		}
	}
	return 0
}

func (c *UniversalChain[M, B]) Recovery(epoch uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if epoch < c.checksumEpoch {
		log.Printf("Recovery request to an epoch before checksum: rollback to %v vs checksum epoch %v", epoch, c.checksumEpoch)
		return
	}
	if epoch > c.stateEpoch {
		log.Printf("Recovery request to an epoch after state epoch: rollback to %v vs state epoch %v", epoch, c.stateEpoch)
		return
	}
	var mutations M
	for n, block := range c.recentBlocks {
		if block.Epoch == c.checksumEpoch+1 {
			mutations = block.mutations

		} else if block.Epoch > c.checksumEpoch+1 && block.Epoch <= epoch {
			mutations.Merge(block.mutations)
		}
		if block.Epoch == epoch {
			c.recentBlocks = c.recentBlocks[:n]
			break
		}
	}
	c.commitState.Recover()
	c.commitState.Incorporate(mutations)
	c.lastCommitEpoch = epoch
	c.blocks = c.blocks[:0]
	c.commitState.Incorporate(mutations)
}

func (c *UniversalChain[M, B]) Sync(conn *socket.CachedConnection, epoch uint64) {
	c.mu.Lock()
	syncBlocks := make([]*ActionBlock[M], 0)
	for _, block := range c.recentBlocks {
		if block.Epoch > epoch && block.Epoch < c.epoch {
			syncBlocks = append(syncBlocks, block)
		}
	}
	syncBlocks = append(syncBlocks, c.liveBlock.Clone())
	c.mu.Unlock()
	for n, block := range syncBlocks {
		bytes := []byte{chain.MsgProtocolNewBlock}
		util.PutUint64(epoch, &bytes)
		conn.SendDirect(bytes)
		bytes = block.Actions.Serialize()
		conn.SendDirect(append([]byte{chain.MsgProtocolActionArray}, bytes...))
		if n == len(syncBlocks)-1 {
			break
		}
		if block.Status >= StatusSealed {
			bytes := []byte{chain.MsgProtocolSealBlock}
			util.PutUint64(epoch, &bytes)
			util.PutHash(block.Actions.Hash(), &bytes)
			conn.SendDirect(bytes)
		}
		if block.Status >= StatusCommit {
			bytes := []byte{chain.MsgProtocolCommitBlock}
			util.PutUint64(epoch, &bytes)
			util.PutHashArray(block.Invalidated, &bytes)
			conn.SendDirect(bytes)
		}
	}
	conn.Ready()
}
