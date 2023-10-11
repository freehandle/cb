package topos

import (
	"fmt"
	"log"
	"net"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/util"

	"github.com/freehandle/breeze/socket"
)

// RelayConfig is the configuration for a relay node.
type RelayConfig struct {
	SourceAddress string                    // url:port
	SourceToken   crypto.Token              // known token of the block provider
	Credentials   crypto.PrivateKey         // secret key of the relay node
	ListenPort    int                       // other nodes must connect to this port
	Validate      socket.ValidateConnection // check if a token is allowed
	Strict        bool                      // all incoming actions must be valid
}

type RelaySyncRequest struct {
	Conn  *socket.CachedConnection
	Token crypto.Token
	Epoch uint64
}

func NewRelay(config RelayConfig, chain *Blockchain) chan error {

	finalize := make(chan error, 2)

	listeners, err := net.Listen("tcp", fmt.Sprintf(":%v", config.ListenPort))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", config.ListenPort, err)
		return finalize
	}

	conn, err := socket.Dial(config.SourceAddress, config.Credentials, config.SourceToken)
	if err != nil {
		finalize <- fmt.Errorf("could not connect to block provider: %v", err)
		return finalize
	}

	if err := conn.Send(NewSyncRequest(chain.state.Epoch())); err != nil {
		finalize <- fmt.Errorf("could not send sync request: %v", err)
		return finalize
	}

	incorporate := make(chan *RelaySyncRequest)
	msg := make(chan []byte)

	pool := make(socket.ConnectionPool)

	go func() {
		for {
			data, err := conn.Read()
			if err != nil {
				finalize <- fmt.Errorf("error reading from block provider: %v", err)
				return
			}
			if data[0] == MsgBlock {
				if len(data) == 1+8+crypto.Size {
					epoch, position := util.ParseUint64(data, 1)
					hash, _ := util.ParseHash(data, position)
					if err := chain.NextBlock(epoch, hash); err != nil {
						finalize <- fmt.Errorf("error processing new block: %v", err)
						return
					} else {
						msg <- data
					}
				} else {
					log.Print("invalid new block message")
				}
			} else if data[0] == MsgAction {
				if len(data) > 1 {
					action := data[1:]
					if err := chain.Append(action); err != nil {
						log.Printf("invalid action: %v", err)
					} else {
						msg <- data
					}
				} else {
					log.Print("invalid action message")
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case request := <-incorporate:
				if request == nil {
					return
				}
				pool[request.Token] = request.Conn
				go chain.Sync(request.Conn, request.Epoch)
			case data := <-msg:
				pool.Broadcast(append([]byte{MsgBlock}, data...))
			}
		}
	}()

	go func() {
		for {
			if conn, err := listeners.Accept(); err == nil {
				trustedConn, err := socket.PromoteConnection(conn, config.Credentials, config.Validate)
				if err != nil {
					conn.Close()
				} else {

					go WaitSyncRequest(trustedConn, incorporate)
				}
			} else {
				return
			}
		}
	}()
	return finalize
}

func WaitSyncRequest(conn *socket.SignedConnection, incorporate chan *RelaySyncRequest) {
	data, err := conn.Read()
	if err != nil {
		conn.Shutdown()
		return
	}
	if data[0] != MsgSyncRequest || len(data) != 9 {
		conn.Shutdown()
	}
	cached := socket.NewCachedConnection(conn)
	request := RelaySyncRequest{
		Conn:  cached,
		Token: conn.Token,
	}
	request.Epoch, _ = util.ParseUint64(data, 1)
	incorporate <- &request
}
