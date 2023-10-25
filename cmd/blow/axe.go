package main

import (
	"fmt"
	"log"

	"github.com/freehandle/axe/attorney"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/cb/social"
	"github.com/freehandle/cb/topos"
	"github.com/freehandle/papirus"
)

func AxeValidator(validator crypto.PrivateKey, source crypto.Token) chan error {
	config := social.ProtocolValidatorNodeConfig{
		BlockProviderAddr:  "localhost:5006",
		BlockProviderToken: source,
		Port:               6000,
		NodeCredentials:    validator,
		ValidateOutgoing:   socket.AcceptAllConnections,
		KeepNBlocks:        100000,
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
