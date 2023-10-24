package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/freehandle/axe/attorney"
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/consensus/poa"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/protocol/actions"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/papirus"
	"github.com/freehandle/safe"

	"github.com/freehandle/cb/blocks"
	"github.com/freehandle/cb/social"
	"github.com/freehandle/cb/topos"
)

// relay
// index
// validator
// block history

var pkHex = "f622f274b13993e3f1824a30ef0f7e57f0c35a4fbdc38e54e37916ef06a64a797eb7aa3582b216bba42d45e91e0a560508478f5b55228439b42733945fd5c2f5"

// Gateway Port 5100
// AxeValidator Port 6000
// AxeBlockProvider Port 6001
// Breeze Incoming 5005
// Breeze Outgoing 5006

func breeze(pk crypto.PrivateKey) chan error {

	config := poa.SingleAuthorityConfig{
		IncomingPort:     5005,
		OutgoingPort:     5006,
		BlockInterval:    time.Second,
		ValidateIncoming: socket.AcceptAllConnections,
		ValidateOutgoing: socket.AcceptAllConnections,
		WalletFilePath:   "", // memory
		KeepBlocks:       50,
		Credentials:      pk,
	}
	return poa.Genesis(config)
}

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
		BlockProviderAddr:  "localhost:5006",
		BlockProviderToken: source,
		Port:               6000,
		NodeCredentials:    validator,
		ValidateOutgoing:   socket.AcceptAllConnections,
		KeepNBlocks:        1000,
	}
	s := attorney.NewGenesisState("")
	chain := social.NewSocialBlockChain[*attorney.Mutations, *attorney.MutatingState](s, 0)
	if chain == nil {
		log.Fatal("could not create axe protocol chain")
	}
	return social.LaunchNode[*attorney.Mutations, *attorney.MutatingState](config, chain)
}

func AxeBlockProvider(provider crypto.PrivateKey, source crypto.Token) chan error {
	finalize := make(chan error, 2)
	storage := papirus.NewFileStore("blocks", 1<<22)
	if storage == nil {
		finalize <- fmt.Errorf("could not open block store")
		return finalize
	}
	store := social.NewBlockStore(storage)
	config := topos.BlockProviderConfig{
		NodeAddress: "localhost:6000",
		NodeToken:   source,
		Credentials: provider,
		ListenPort:  6001,
		Validate:    socket.AcceptAllConnections,
		Store:       store,
	}
	return topos.BlockProviderNode(config)
}

func safeServer(gateway crypto.Token, axe crypto.Token, safePK crypto.PrivateKey, path string) chan error {
	config := safe.SafeConfig{
		GatewayAddress: "localhost:5100",
		GatewayToken:   gateway,
		AxeAddress:     "localhost:6000",
		AxeToken:       axe,
		Credentials:    safePK,
		Port:           7100,
	}
	return safe.NewServer(config, path)

}

func main() {
	var safepath string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "SAFE=") {
			safepath = strings.TrimPrefix(env, "SAFE=")
			break
		}

	}
	var pk crypto.PrivateKey
	bytesPk, _ := hex.DecodeString(pkHex)
	copy(pk[:], bytesPk)
	_, myCredentials := crypto.RandomAsymetricKey()

	_, axeValidatorCredentials := crypto.RandomAsymetricKey()
	_, axeProviderCredentials := crypto.RandomAsymetricKey()
	_, safeCredentials := crypto.RandomAsymetricKey()

	breezeerr := breeze(pk)
	time.Sleep(200 * time.Millisecond)
	store, blockErr := Blocks(pk.PublicKey())
	time.Sleep(200 * time.Millisecond)
	gatewayErr := Gateway(myCredentials, pk.PublicKey())
	time.Sleep(200 * time.Millisecond)
	axenode := AxeValidator(axeValidatorCredentials, pk.PublicKey())
	time.Sleep(200 * time.Millisecond)
	axeProvider := AxeBlockProvider(axeProviderCredentials, axeValidatorCredentials.PublicKey())
	time.Sleep(200 * time.Millisecond)
	socailListener := testListener(axeProviderCredentials.PublicKey(), axeValidatorCredentials.PublicKey())
	time.Sleep(200 * time.Millisecond)
	safeServer := safeServer(myCredentials.PublicKey(), axeValidatorCredentials.PublicKey(), safeCredentials, safepath)
	time.Sleep(200 * time.Millisecond)

	go ListenAndServe(store) // block listener

	//go transferspacket(pk, myCredentials.PublicKey())
	//go axetest(pk, myCredentials.PublicKey())
	select {
	case err := <-blockErr:
		log.Fatalf("block store unrecovarable error: %s", err)
	case err := <-gatewayErr:
		log.Fatalf("gateway unrecovarable error: %s", err)
	case err := <-axenode:
		log.Fatalf("axe node unrecovarable error: %s", err)
	case err := <-axeProvider:
		log.Fatalf("axe block provider unrecovarable error: %s", err)
	case err := <-socailListener:
		log.Fatalf("social listener unrecovarable error: %s", err)
	case err := <-breezeerr:
		log.Fatalf("breeze unrecovarable error: %s", err)
	case err := <-safeServer:
		log.Fatalf("safe server unrecovarable error: %s", err)
	}
}

func axetest(credentials crypto.PrivateKey, node crypto.Token) {
	_, key := crypto.RandomAsymetricKey()
	conn, err := socket.Dial("localhost:5100", key, node)
	if err != nil {
		log.Fatal(err)
	}
	authors := make([]crypto.PrivateKey, 10000)
	for n := 0; n < 10000; n++ {
		_, authors[n] = crypto.RandomAsymetricKey()
		action := attorney.JoinNetwork{
			Epoch:  0,
			Author: authors[n].PublicKey(),
			Handle: fmt.Sprintf("user_%d", n),
		}
		action.Sign(authors[n])
		data := action.Serialize()
		data = actions.Dress(data, authors[n], 0)
		err := conn.Send(append([]byte{chain.MsgActionSubmit}, data...))
		if err != nil {
			log.Fatalf("axetest:%v", err)
		}
	}
	for n := 100; n < 10000; n++ {
		action := attorney.GrantPowerOfAttorney{
			Epoch:       10,
			Author:      authors[n].PublicKey(),
			Attorney:    authors[n%100].PublicKey(),
			Fingerprint: make([]byte, 0),
		}
		action.Sign(authors[n])
		data := action.Serialize()
		data = actions.Dress(data, authors[n], 0)
		err := conn.Send(append([]byte{chain.MsgActionSubmit}, data...))
		if err != nil {
			log.Fatalf("axetest:%v", err)
		}
	}
	teste := []byte("string para testar o void")
	for n := 100; n < 10000; n++ {
		action := attorney.Void{
			Epoch:  20,
			Author: authors[n].PublicKey(),
			Data:   teste,
			Signer: authors[n%100].PublicKey(),
		}
		action.Sign(authors[n%100])
		data := action.Serialize()
		data = actions.Dress(data, authors[n], 0)
		err := conn.Send(append([]byte{chain.MsgActionSubmit}, data...))
		if err != nil {
			log.Fatalf("axetest:%v", err)
		}

	}
	for n := 100; n < 10000; n++ {
		action := attorney.RevokePowerOfAttorney{
			Epoch:    30,
			Author:   authors[n].PublicKey(),
			Attorney: authors[n%100].PublicKey(),
		}
		action.Sign(authors[n])
		data := action.Serialize()
		data = actions.Dress(data, authors[n], 0)
		err := conn.Send(append([]byte{chain.MsgActionSubmit}, data...))
		if err != nil {
			log.Fatalf("axetest:%v", err)
		}
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
			log.Fatalf("transferpacket:%v", err)
		}
		time.Sleep(1 * time.Millisecond)
	}
}

func testListener(provider, node crypto.Token) chan error {
	_, listener := crypto.RandomAsymetricKey()
	newBlock := make(chan *social.ProtocolBlock)
	buffer := topos.NewBlockSorter(1, newBlock)
	config := topos.SyncSocialConfig{
		ProviderAddress: "localhost:6001",
		ProviderToken:   provider,
		NodeAddress:     "localhost:6000",
		NodeToken:       node,
		Credentials:     listener,
		BlockStore:      buffer,
	}

	go func() {
		for {
			block := <-newBlock
			if block != nil {
				fmt.Println("social listener:", block.Epoch, len(block.Actions))
			} else {
				fmt.Println("social listener: nil")
			}
		}
	}()

	return topos.SyncSocial(config)

}
