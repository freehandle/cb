package main

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/freehandle/axe/attorney"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/cb/social"
)

var pkHex = "f622f274b13993e3f1824a30ef0f7e57f0c35a4fbdc38e54e37916ef06a64a797eb7aa3582b216bba42d45e91e0a560508478f5b55228439b42733945fd5c2f5"

func main() {
	var pk crypto.PrivateKey
	bytesPk, _ := hex.DecodeString(pkHex)
	copy(pk[:], bytesPk)

	config := social.ProtocolValidatorNodeConfig{
		BlockProviderAddr:  "localhost:5005",
		BlockProviderToken: pk.PublicKey(),
		Port:               6000,
		NodeCredentials:    pk,
		ValidateOutgoing:   socket.AcceptAllConnections,
		KeepNBlocks:        1000,
	}

	s := attorney.NewGenesisState("")
	chain := social.NewSocialBlockChain[*attorney.Mutations, *attorney.MutatingState](s, 0)
	if chain == nil {
		log.Fatal("wrong")
	}
	err := <-social.LaunchNode[*attorney.Mutations, *attorney.MutatingState](config, chain)
	fmt.Println(err)
}
