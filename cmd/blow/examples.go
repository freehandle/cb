package main

import (
	"fmt"
	"log"
	"time"

	"github.com/freehandle/axe/attorney"
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/protocol/actions"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/cb/social"
	"github.com/freehandle/cb/topos"
)

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
