package main

import (
	"time"

	"github.com/freehandle/breeze/consensus/poa"
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
)

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
