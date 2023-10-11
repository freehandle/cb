package topos

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
)

type GatewayConfig struct {
	NodeAddress string
	NodeToken   crypto.Token
	Credentials crypto.PrivateKey
	ListenPort  int
	Validate    socket.ValidateConnection
}

type GatewayConnection struct {
	Conn *socket.SignedConnection
	Live bool
}

func NewGateway(config GatewayConfig) chan error {

	finalize := make(chan error, 2)

	listeners, err := net.Listen("tcp", fmt.Sprintf(":%v", config.ListenPort))
	if err != nil {
		finalize <- fmt.Errorf("could not listen on port %v: %v", config.ListenPort, err)
		return finalize
	}

	conn, err := socket.Dial(config.NodeAddress, config.Credentials, config.NodeToken)
	if err != nil {
		finalize <- fmt.Errorf("could not connect to block provider: %v", err)
		return finalize
	}

	action := make(chan []byte)
	connection := make(chan GatewayConnection)

	live := make(map[crypto.Token]*socket.SignedConnection)
	shutdown := false

	lock := sync.Mutex{}

	go func() {
		for {
			if conn, err := listeners.Accept(); err == nil {
				trustedConn, err := socket.PromoteConnection(conn, config.Credentials, config.Validate)
				if err != nil {
					conn.Close()
				} else {
					go WaitForActions(trustedConn, connection, action)
				}
			} else {
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case connection := <-connection:
				lock.Lock()
				if connection.Live {
					live[connection.Conn.Token] = connection.Conn
				} else {
					delete(live, connection.Conn.Token)
				}
				lock.Unlock()
				if shutdown && len(live) == 0 {
					finalize <- nil
					return
				}
			case data := <-action:
				if len(data) == 0 {
					lock.Lock()
					shutdown = true
					for _, conn := range live {
						conn.Shutdown()
					}
					lock.Unlock()
				}
				if err := conn.Send(append([]byte{MsgActionSubmit}, data...)); err != nil {
					log.Printf("could not send action to block provider: %v", err)
				}
			}
		}
	}()

	return finalize
}

func WaitForActions(conn *socket.SignedConnection, terminate chan GatewayConnection, action chan []byte) {
	for {
		data, err := conn.Read()
		if err != nil || len(data) < 2 || data[0] != MsgActionSubmit {
			conn.Shutdown()
			terminate <- GatewayConnection{Conn: conn, Live: false}
			return
		}
		action <- data[1:]
	}
}
