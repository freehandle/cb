package topos

import (
	"fmt"
	"net"
	"time"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/breeze/util"
)

type SingleAuthorityConfig struct {
	IncomingPort     int
	OutgoingPort     int
	Credentials      crypto.PrivateKey
	BlockInterval    time.Duration
	ValidateIncoming socket.ValidateConnection
	ValidateOutgoing socket.ValidateConnection
	KeepBlocks       int
}

type OutgoindConnectionRequest struct {
	conn  *socket.SignedConnection
	epoch uint64
}

func NewSingleAuthority(config SingleAuthorityConfig, state ProtocolState) chan error {

	finalize := make(chan error, 2)

	incomming, err := net.Listen("tcp", fmt.Sprintf(":%v", config.IncomingPort))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", config.IncomingPort, err)
		return finalize
	}

	outgoing, err := net.Listen("tcp", fmt.Sprintf(":%v", config.OutgoingPort))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", config.OutgoingPort, err)
		return finalize
	}

	endIncomming := make(chan crypto.Token)
	newIncoming := make(chan *socket.SignedConnection)
	incomingConnections := make(map[crypto.Token]*socket.SignedConnection)

	newOutgoing := make(chan OutgoindConnectionRequest)

	action := make(chan []byte)
	incorporated := make(chan []byte)
	newBlock := make(chan uint64)

	blocks := NewRecentBlocks(config.KeepBlocks, state.Epoch())

	ticker := time.NewTicker(config.BlockInterval)

	pool := make(socket.ConnectionPool)
	// listen incomming
	go func() {
		for {
			if conn, err := incomming.Accept(); err == nil {
				trustedConn, err := socket.PromoteConnection(conn, config.Credentials, config.ValidateIncoming)
				if err != nil {
					conn.Close()
				}
				newIncoming <- trustedConn
			}
		}
	}()

	// manage incoming connections and block formation
	go func() {
		for {
			select {
			case token := <-endIncomming:
				delete(incomingConnections, token)
			case conn := <-newIncoming:
				incomingConnections[conn.Token] = conn
				go WaitForProtocolActions(conn, endIncomming, action)
			case proposed := <-action:
				if err := state.Action(proposed); err != nil {
					incorporated <- proposed
				}
			case <-ticker.C:
				epoch := state.Epoch() + 1
				state.NextBlock(epoch)
				newBlock <- epoch
			}

		}
	}()

	go func() {
		for {
			select {
			case epoch := <-newBlock:
				hash := blocks.current.Hash()
				blocks.NextBlock()
				data := []byte{MsgBlock}
				util.PutUint64(epoch, &data)
				util.PutHash(hash, &data)
				pool.DropDead() // clear dead connections
				pool.Broadcast(data)
			case action := <-incorporated:
				data := []byte{MsgAction}
				data = append(data, action...)
				pool.Broadcast(data)
			case req := <-newOutgoing:
				cached := socket.NewCachedConnection(req.conn)
				pool.Add(cached)
				go blocks.Sync(cached, req.epoch)
			}
		}

	}()

	// listen outgoing (cached with recent blocks)
	go func() {
		for {
			if conn, err := outgoing.Accept(); err == nil {
				trustedConn, err := socket.PromoteConnection(conn, config.Credentials, config.ValidateIncoming)
				if err != nil {
					conn.Close()
				}
				go WaitForOutgoingSyncRequest(trustedConn, newOutgoing)
			}

		}
	}()

	return finalize
}

func WaitForOutgoingSyncRequest(conn *socket.SignedConnection, outgoing chan OutgoindConnectionRequest) {
	data, err := conn.Read()
	if err != nil || len(data) != 9 || data[0] != MsgSyncRequest {
		conn.Shutdown()
		return
	}
	epoch, _ := util.ParseUint64(data, 1)
	outgoing <- OutgoindConnectionRequest{conn: conn, epoch: epoch}
}

func WaitForProtocolActions(conn *socket.SignedConnection, terminate chan crypto.Token, action chan []byte) {
	for {
		data, err := conn.Read()
		if err != nil || len(data) < 2 || data[0] != MsgActionSubmit {
			conn.Shutdown()
			terminate <- conn.Token
			return
		}
		action <- data[1:]
	}
}
