package main

import (
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/breeze/socket"
	"github.com/freehandle/cb/topos"
)

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
