package topos

import (
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
	"github.com/freehandle/cb/social"
)

const buffersize = 1 >> 24 // 16MB

type BlockProviderConfig struct {
	NodeAddress string
	NodeToken   crypto.Token
	Credentials crypto.PrivateKey
	ListenPort  int
	Validate    socket.ValidateConnection
	Store       *social.BlockStore
}

func BlockProviderNode(config BlockProviderConfig) chan error {
	finalize := make(chan error, 2)

	listeners, err := net.Listen("tcp", fmt.Sprintf(":%v", config.ListenPort))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", config.ListenPort, err)
		return finalize
	}

	source, err := socket.Dial(config.NodeAddress, config.Credentials, config.NodeToken)
	if err != nil {
		finalize <- fmt.Errorf("could not connect to block provider node %v: %v", config.NodeAddress, err)
	}

	go func() {
		notCommit := make(map[uint64]*social.ProtocolBuilder)
		var block *social.ProtocolBuilder
		for {
			data, err := source.Read()
			if err != nil {
				listeners.Close() // force finalize of listener connection
				return
			}
			if len(data) == 0 {
				continue
			}
			switch data[0] {
			case chain.MsgNewBlock:
				epoch, _ := util.ParseUint64(data, 1)
				block = social.NewProtocolBuilder(epoch)
			case chain.MsgAction:
				action, _ := util.ParseByteArray(data, 1)
				if block != nil {
					block.AddAction(action)
				} else {
					log.Printf("BlockProviderNode, action received, but no active block")
				}
			case chain.MsgBlockSealed:
				epochSeal, position := util.ParseUint64(data, 1)
				var hash crypto.Hash
				hash, _ = util.ParseHash(data, position)
				if block, ok := notCommit[epochSeal]; ok && block.Building() {
					seal := block.Seal()
					if !seal.Equal(hash) {
						log.Printf("BlockProviderNode, block seal does not match hash: %v", hash)
					}
				} else {
					log.Printf("BlockProviderNode, block seal received for epoch %v, but no block found", epochSeal)
				}
			case chain.MsgBlockCommitted:
				epochCommit, position := util.ParseUint64(data, 1)
				invalidated, _ := util.ParseHashArray(data, position)
				if block, ok := notCommit[epochCommit]; ok && block.Sealed() {
					block.Finalize(invalidated, config.Credentials)
					if err := config.Store.AddBlock(block.Bytes()); err == nil {
						delete(notCommit, epochCommit)
					} else {
						log.Printf("BlockProviderNode, could not add block to block store: %v", err)
					}
				} else {
					log.Printf("BlockProviderNode, block commit received for epoch %v, but no block found", epochCommit)
				}
			}
		}
	}()

	go func() {
		for {
			conn, err := listeners.Accept()
			if err != nil {
				finalize <- err
				return
			}
			trustedConn, err := socket.PromoteConnection(conn, config.Credentials, config.Validate)
			if err != nil {
				conn.Close()
			} else {
				go TransmitBlocks(trustedConn, config.Store)
			}
		}
	}()

	return finalize
}

func NewBlockProvider(port int, pk crypto.PrivateKey, validate socket.ValidateConnection, store *social.BlockStore) chan error {
	finalize := make(chan error, 2)
	listeners, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", port, err)
		return finalize
	}

	go func() {
		for {
			conn, err := listeners.Accept()
			if err != nil {
				finalize <- err
				return
			}
			trustedConn, err := socket.PromoteConnection(conn, pk, validate)
			if err != nil {
				conn.Close()
			} else {
				go TransmitBlocks(trustedConn, store)
			}
		}
	}()

	return nil
}

func byteArrayToBlockArray(data []byte) []*social.ProtocolBlock {
	position := 0
	protocols := make([]*social.ProtocolBlock, 0)
	for {
		var protocol *social.ProtocolBlock
		protocol, position = social.ParseProtocolBlockWithPosition(data, position)
		if protocol != nil {
			protocols = append(protocols, protocol)
		} else {
			log.Print("could not parse blocks from data")
			return nil
		}
		if position >= len(data) {
			return protocols
		}
	}
}

func ReceiveBlocks(address string, token crypto.Token, pk crypto.PrivateKey, epoch uint64) (chan *social.ProtocolBlock, uint64, error) {
	if epoch == 0 {
		return nil, 0, errors.New("epoch 0 is reserved for genesis block. start sync at epoch 1")
	}
	output := make(chan *social.ProtocolBlock, 2)
	conn, err := socket.Dial(address, pk, token)
	if err != nil {
		return nil, 0, err
	}
	bytes, err := conn.Read()
	if err != nil {
		return nil, 0, err
	}
	lastEpoch, _ := util.ParseUint64(bytes, 0)
	if lastEpoch < epoch {
		output <- nil // signal end of transmission
		return output, lastEpoch, nil
	}
	go func() {
		defer conn.Shutdown()
		for {
			data, err := conn.Read()
			if err != nil {
				output <- nil
				return
			}
			// invalid transmission
			if len(data) < 8 {
				conn.Shutdown()
				log.Printf("ReceiveBlocks, could not receive blocks from connection token %v: %v", conn.Token, err)
				output <- nil
			}
			// end of transmission signal
			if len(data) == 8 && data[0]+data[1]+data[2]+data[3]+data[4]+data[5]+data[6]+data[7] == 0 {
				if epoch != lastEpoch {
					log.Printf("ReceiveBlocks, could not receive all blocks. Last %v instead of %v", epoch, lastEpoch)
				}
				output <- nil
				return
			}
			// parse blocks
			blocks := byteArrayToBlockArray(data)
			if blocks == nil {
				log.Printf("ReceiveBlocks, could not parse blocks from data")
				output <- nil
				return
			}
			for _, block := range blocks {
				if epoch == block.Epoch {
					output <- block
					if epoch < lastEpoch {
						epoch = epoch + 1
					}
				} else {
					log.Printf("ReceiveBlocks, blocks out of order %v instead of %v", block.Epoch, epoch)
				}
			}
		}
	}()
	return output, lastEpoch, nil
}

// Transmit clocks send to conn requested blocks from store.
// The transmitter sends the receiver 8 bytes indicating the last epoch for which
// there are blocks available. It then waits the receiver to send 8 bytes indicating
// the first epoch the receiver wants blocks to be transmitted.
// If the receiver requests blocks from an epoch that is greater than the last epoch
// the transmiteer send 8 zero bytes indicating there are no blocks to be transmitted.
// Other
func TransmitBlocks(conn *socket.SignedConnection, store *social.BlockStore) {
	defer conn.Shutdown()
	last := store.Epoch
	bytes := make([]byte, 0)
	util.PutUint64(last, &bytes)
	err := conn.Send(bytes)
	if err != nil {
		log.Printf("BroadcastBlock, could not send last epoch to connection token %v: %v", conn.Token, err)
		return
	}
	bytes, err = conn.Read()
	if err != nil {
		log.Printf("BroadcastBlock, could not receive sync from connection token %v: %v", conn.Token, err)
		return
	}
	start, _ := util.ParseUint64(bytes, 0)
	if start > last {
		zero := make([]byte, 8)
		conn.Send(zero)
		return
	}
	if start == 0 {
		start = 1 // block 0 is exclusive for breeze genesis block
	}
	buffer := make([]byte, 0)
	for epoch := start; epoch <= last; epoch++ {
		block := store.GetBlock(int(epoch))
		buffer = append(buffer, block...)
		if len(buffer) > buffersize || epoch == last {
			err := conn.Send(buffer)
			if err != nil {
				log.Printf("BroadcastBlock, could not send blocks to connection token %v: %v", conn.Token, err)
				return
			}
			buffer = buffer[:0]
		}
	}
}
