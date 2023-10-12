package topos

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/protocol/actions"
	"github.com/freehandle/breeze/socket"
)

type Dresser interface {
	Dress([]byte) []byte
}

func NewBreezeVoidDresser(pk crypto.PrivateKey, fee uint64) *BreezeVoidDresseer {
	return &BreezeVoidDresseer{
		secret: pk,
		wallet: pk.PublicKey(),
		fee:    fee,
	}
}

type BreezeVoidDresseer struct {
	secret crypto.PrivateKey
	wallet crypto.Token
	fee    uint64
}

func (b *BreezeVoidDresseer) SetFee(fee uint64) {
	b.fee = fee
}

func (b *BreezeVoidDresseer) Dress(data []byte) []byte {
	if actions.Kind(data) == actions.IVoid {
		void := actions.ParseVoid(data)
		if void == nil {
			return data
		}
		void.Wallet = b.wallet
		void.Fee = b.fee
		void.Sign(b.secret)
		return void.Serialize()
	}
	return data
}

type GatewayConfig struct {
	NodeAddress string
	NodeToken   crypto.Token
	Credentials crypto.PrivateKey
	ListenPort  int
	Validate    socket.ValidateConnection
	Dresser     Dresser
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

	fmt.Printf("gateway trying to connect to block provider: %v\n", config.NodeAddress)
	conn, err := socket.Dial(config.NodeAddress, config.Credentials, config.NodeToken)
	if err != nil {
		finalize <- fmt.Errorf("could not connect to block provider: %v", err)
		return finalize
	}
	fmt.Println("gateway connected to block provider")
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
					fmt.Println("gateway accepted connection")
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
				if config.Dresser != nil {
					data = config.Dresser.Dress(data)
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
