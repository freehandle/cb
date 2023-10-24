package main

import (
	"github.com/freehandle/breeze/consensus/chain"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/cb/blocks"
	"github.com/freehandle/papirus"
)

// blockchain storage
func Blocks(pk crypto.PrivateKey, node crypto.Token) (*blocks.BlockStore, chan error) {
	bytes := papirus.NewMemoryStore(1e6)
	store := blocks.BlockStore{
		Storage:  []papirus.ByteStore{bytes},
		Blocks:   make([]blocks.BlockIndex, 0),
		Current:  nil,
		Unselaed: make([]*chain.BlockBuilder, 0),
		Sealed:   make([]*chain.SealedBlock, 0),
	}
	config := blocks.BlockListenerConfig{
		//NodeAddr:    "192.168.15.83:5006",
		NodeAddr:    "localhost:5006",
		NodeToken:   node,
		Credentials: pk,
	}
	finalize := blocks.NewBlockListener(config, &store)
	return &store, finalize
}
