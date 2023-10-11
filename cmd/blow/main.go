package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/protocol/actions"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/papirus"

	"github.com/freehandle/cb/blocks"
	"github.com/freehandle/cb/topos"
)

// relay
// index
// validator
// block history

var pkHex = "f622f274b13993e3f1824a30ef0f7e57f0c35a4fbdc38e54e37916ef06a64a797eb7aa3582b216bba42d45e91e0a560508478f5b55228439b42733945fd5c2f5"

func Blocks(node crypto.Token) {
	time.Sleep(5 * time.Second)
	bytes := papirus.NewMemoryStore(1e6)
	store := blocks.BlockStore{
		Storage:  []papirus.ByteStore{bytes},
		Blocks:   make([]blocks.BlockIndex, 0),
		Current:  nil,
		Unselaed: make([]*chain.BlockBuilder, 0),
		Sealed:   make([]*chain.SealedBlock, 0),
	}
	_, pk := crypto.RandomAsymetricKey()
	config := blocks.BlockListenerConfig{
		NodeAddr:    "localhost:5006",
		NodeToken:   node,
		Credentials: pk,
	}
	blocks.NewBlockListener(config, &store)
}

func main() {
	var pk crypto.PrivateKey
	bytesPk, _ := hex.DecodeString(pkHex)
	copy(pk[:], bytesPk)

	_, myCredentials := crypto.RandomAsymetricKey()

	config := topos.GatewayConfig{
		NodeAddress: "localhost:5005",
		NodeToken:   pk.PublicKey(),
		Credentials: myCredentials,
		ListenPort:  5100,
		Validate:    socket.AcceptAllConnections,
	}
	gateway := topos.NewGateway(config)
	go func() {
		_, walletKey := crypto.RandomAsymetricKey()
		conn, err := socket.Dial("localhost:5100", walletKey, myCredentials.PublicKey())
		if err != nil {
			log.Fatal(err)
		}
		for n := 0; n < 100000; n++ {
			token, _ := crypto.RandomAsymetricKey()
			transfer := actions.Transfer{
				TimeStamp: uint64(n / 1000),
				From:      pk.PublicKey(),
				To:        []crypto.TokenValue{{Token: token, Value: 1}},
				Reason:    "whatever",
				Fee:       1,
			}
			transfer.Sign(pk)
			action := transfer.Serialize()
			err := conn.Send(append([]byte{chain.MsgActionSubmit}, action...))
			if err != nil {
				log.Print(err)
			}
			time.Sleep(1 * time.Millisecond)
		}

	}()
	Blocks(pk.PublicKey())
	fmt.Println(<-gateway)
}
