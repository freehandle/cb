package main

import (
	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/safe"
)

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
