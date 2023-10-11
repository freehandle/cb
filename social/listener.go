package social

import (
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/util"

	"github.com/freehandle/breeze/socket"
)

const (
	ErrSignal byte = iota
	NewBlockSignal
	ActionSignal
	ActionArraySignal
	SealSignal
	CommitSignal
)

type BlockSignal struct {
	Signal     byte
	Epoch      uint64
	Checkpoint uint64
	Hash       crypto.Hash
	HashArray  []crypto.Hash
	Token      crypto.Token
	Signature  crypto.Signature
	Action     []byte
	Actions    *chain.ActionArray
	Err        error
}

func SocialProtocolBlockListener(config ProtocolValidatorNodeConfig, epoch uint64, signal *BlockSignal) chan byte {
	send := make(chan byte)

	return send
}

// Connects to a node providing breeze new blocks and forward signals to the channel
func BreezeBlockListener(config ProtocolValidatorNodeConfig, epoch uint64, signal *BlockSignal) chan byte {
	send := make(chan byte)
	conn, err := socket.Dial(config.BlockProviderAddr, config.NodeCredentials, config.BlockProviderToken)
	if err != nil {
		signal.Err = err
		send <- ErrSignal
		return send
	}
	go func() {
		conn.Send(chain.SyncMessage(epoch))
		for {
			msg, err := conn.Read()
			if err != nil {
				signal.Err = err
				send <- ErrSignal
				return
			}
			switch msg[0] {
			case chain.MsgAction:
				signal.Action = msg[1:]
				send <- ActionSignal
			case chain.MsgNewBlock:
				header := chain.ParseBlockHeader(msg[1:])
				if header != nil {
					signal.Epoch = header.Epoch
					signal.Checkpoint = header.CheckPoint
					signal.Hash = header.CheckpointHash
					signal.Token = header.Proposer
					send <- NewBlockSignal
				}
			case chain.MsgSealBlock:
				if len(msg) > 9 {
					signal.Epoch, _ = util.ParseUint64(msg, 1)
					seal := chain.ParseBlockSeal(msg[9:])
					if seal != nil {
						signal.Hash = seal.Hash
						signal.Signature = seal.SealSignature
						send <- SealSignal
					}
				}
			case chain.MsgCommitBlock:
				if len(msg) > 9 {
					signal.Epoch, _ = util.ParseUint64(msg, 1)
					commit := chain.ParseBlockCommit(msg[9:])
					if commit != nil {
						signal.HashArray = commit.Invalidated
						signal.Token = commit.PublishedBy
						signal.Signature = commit.PublishSign
						send <- CommitSignal
					}
				}
			case chain.MsgBlockCommitted:
				block := chain.ParseCommitBlock(msg[1:])
				if block != nil {
					signal.Epoch = block.Header.Epoch
					signal.Checkpoint = block.Header.CheckPoint
					signal.Hash = block.Header.CheckpointHash
					signal.Token = block.Header.Proposer
					send <- NewBlockSignal
					signal.Actions = block.Actions
					send <- ActionArraySignal
					signal.Hash = block.Seal.Hash
					signal.Signature = block.Seal.SealSignature
					send <- SealSignal
					signal.HashArray = block.Commit.Invalidated
					signal.Token = block.Commit.PublishedBy
					signal.Signature = block.Commit.PublishSign
					send <- CommitSignal
				}
			case chain.MsgBlockSealed:
				block := chain.ParseSealedBlock(msg[1:])
				if block != nil {
					signal.Epoch = block.Header.Epoch
					signal.Checkpoint = block.Header.CheckPoint
					signal.Hash = block.Header.CheckpointHash
					signal.Token = block.Header.Proposer
					send <- NewBlockSignal
					signal.Actions = block.Actions
					send <- ActionArraySignal
					signal.Hash = block.Seal.Hash
					signal.Signature = block.Seal.SealSignature
					send <- SealSignal
				}
			}
		}
	}()
	return send
}
