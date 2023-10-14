package topos

import (
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

	listeners, err := net.Listen("tcp", fmt.Sprintf(":%v", port))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", port, err)
		return finalize
	}

	source, err := socket.Dial(config.NodeAddress, config.Credentials, config.NodeToken)
	if err != nil {
		finalize <- fmt.Errorf("could not connect to block provider node %v: %v", config.NodeAddress, err)
	}

	

	go func() {
		notCommit := make(chan []byte, 0)
		block := make([]byte,0)
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

func ReceiveBlocks(address string, token crypto.Token, pk crypto.PrivateKey, epoch uint64) (chan []byte, error) {
	output := make(chan []byte, 2)
	conn, err := socket.Dial(address, pk, token)
	if err != nil {
		return nil, err
	}
	bytes, err := conn.Read()
	if err != nil {
		return nil, err
	}
	lastEpoch, _ := util.ParseUint64(bytes, 0)
	if lastEpoch < epoch {
		output <- nil
		return output, nil
	}
	go func() {
		for n := epoch; n <= lastEpoch; n++ {
			blocks, err := conn.Read()
			if err != nil {
				output <- nil
				return
			}
			if len(blocks) < 8 {
				conn.Shutdown()
				log.Printf("ReceiveBlocks, could not receive blocks from connection token %v: %v", conn.Token, err)
				output <- nil
			}
			if len(blocks) == 8 {
				output <- nil
				return
			}
			count, position := util.ParseUint64(blocks, 0)
			for n := uint64(0); n < count; n++ {
				block, position := util.ParseByteArray(blocks, position)
				output <- block
				if position >= len(blocks) {
					break
				}
			}
		}
		output <- nil // signal end of transmission
		bytes, err := conn.Read()
		if err != nil || len(bytes) != 8 || (bytes[0]+bytes[1]+bytes[2]+bytes[3]+bytes[4]+bytes[5]+bytes[6]+bytes[7] > 0) {
			log.Printf("ReceiveBlocks, could not receive confirmation of end of transmission from token %v", conn.Token)
			return
		}
	}()
	return output, nil
}

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
	buffer := make([]byte, 0)
	bufferCount := 0
	for epoch := start; epoch <= last; epoch++ {
		block := store.GetBlock(int(epoch))
		buffer = append(buffer, block...)
		bufferCount += 1
		if len(buffer) > buffersize || epoch == last {
			bytes := make([]byte, 0)
			util.PutUint64(uint64(bufferCount), &bytes)
			buffer = append(bytes, buffer...)
			err := conn.Send(buffer)
			if err != nil {
				log.Printf("BroadcastBlock, could not send blocks to connection token %v: %v", conn.Token, err)
				return
			}
			buffer = buffer[:0]
			bufferCount = 0
		}
	}
}
