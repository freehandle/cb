package main

import (
	"encoding/hex"
	"log"
	"time"

	"github.com/freehandle/axe/attorney"
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/protocol/actions"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/papirus"

	"github.com/freehandle/cb/blocks"
	"github.com/freehandle/cb/social"
	"github.com/freehandle/cb/topos"
)

// relay
// index
// validator
// block history

var pkHex = "f622f274b13993e3f1824a30ef0f7e57f0c35a4fbdc38e54e37916ef06a64a797eb7aa3582b216bba42d45e91e0a560508478f5b55228439b42733945fd5c2f5"

// blockchain storage
func Blocks(node crypto.Token) (*blocks.BlockStore, chan error) {
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
		//NodeAddr:    "192.168.15.83:5006",
		NodeAddr:    "localhost:5006",
		NodeToken:   node,
		Credentials: pk,
	}
	finalize := blocks.NewBlockListener(config, &store)
	return &store, finalize
}

func Gateway(credentials crypto.PrivateKey, node crypto.Token) chan error {

	config := topos.GatewayConfig{
		//NodeAddress: "192.168.15.83:5005",
		NodeAddress: "localhost:5005",
		NodeToken:   node,
		Credentials: credentials,
		ListenPort:  5100,
		Validate:    socket.AcceptAllConnections,
		Dresser:     topos.NewBreezeVoidDresser(credentials, 0),
	}
	return topos.NewGateway(config)
}

func AxeValidator(validator crypto.PrivateKey, source crypto.Token) chan error {
	config := social.ProtocolValidatorNodeConfig{
		BlockProviderAddr:  "localhost:5005",
		BlockProviderToken: source,
		Port:               6000,
		NodeCredentials:    validator,
		ValidateOutgoing:   socket.AcceptAllConnections,
		KeepNBlocks:        1000,
	}
	s := attorney.NewGenesisState("")
	chain := social.NewUniversalChain[*attorney.Mutations, *attorney.MutatingState](s, 0)
	if chain == nil {
		log.Fatal("could not create axe protocol chain")
	}
	return social.LaunchNode[*attorney.Mutations, *attorney.MutatingState](config, chain)
}

func AxeBlockProvider(provider crypto.PrivateKey, source crypto.Token) chan error {

}

func main() {
	var pk crypto.PrivateKey
	bytesPk, _ := hex.DecodeString(pkHex)
	copy(pk[:], bytesPk)
	_, myCredentials := crypto.RandomAsymetricKey()

	store, blockErr := Blocks(pk.PublicKey())
	gatewayErr := Gateway(myCredentials, pk.PublicKey())
	go ListenAndServe(store) // block listener

	//go transferspacket(pk, myCredentials.PublicKey())

	select {
	case err := <-blockErr:
		log.Fatalf("block store unrecovarable error: %s", err)
	case err := <-gatewayErr:
		log.Fatalf("gateway unrecovarable error: %s", err)
	}
}

func transferspacket(credentials crypto.PrivateKey, node crypto.Token) {
	_, walletKey := crypto.RandomAsymetricKey()
	conn, err := socket.Dial("localhost:5100", walletKey, node)
	if err != nil {
		log.Fatal(err)
	}
	for n := 0; n < 100000; n++ {
		token, _ := crypto.RandomAsymetricKey()
		transfer := actions.Transfer{
			TimeStamp: uint64(n / 1000),
			From:      credentials.PublicKey(),
			To:        []crypto.TokenValue{{Token: token, Value: 1}},
			Reason:    "whatever",
			Fee:       1,
		}
		transfer.Sign(credentials)
		action := transfer.Serialize()
		err := conn.Send(append([]byte{chain.MsgActionSubmit}, action...))
		if err != nil {
			log.Print(err)
		}
		time.Sleep(1 * time.Millisecond)
	}
}
