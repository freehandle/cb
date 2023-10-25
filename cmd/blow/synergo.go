package main

import (
	"errors"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/cb/vault"
	"github.com/freehandle/synergy/api"
	"github.com/freehandle/synergy/network"
	"github.com/freehandle/synergy/social/index"
	"github.com/freehandle/synergy/social/state"
)

func synergyApp(gatewayToken crypto.Token, axeNodeToken crypto.Token, credentials, ephemeral crypto.PrivateKey, emailpassword, path string) chan error {
	indexer := index.NewIndex()
	genesis := state.GenesisState(indexer)
	indexer.SetState(genesis)
	//_, attorneySecret := crypto.RandomAsymetricKey()

	gateway := make(chan []byte)

	vault := vault.SecureVault{
		Secrets: make(map[crypto.Token]crypto.PrivateKey),
	}
	vault.Secrets[credentials.PublicKey()] = credentials
	vault.Secrets[ephemeral.PublicKey()] = ephemeral

	cookieStore := api.OpenCokieStore("synergycookies.dat", genesis)
	passwordManager := api.NewFilePasswordManager("synergypasswords.dat")

	config := api.ServerConfig{
		Vault:         &vault,
		Attorney:      credentials.PublicKey(),
		Ephemeral:     ephemeral.PublicKey(),
		Passwords:     passwordManager,
		CookieStore:   cookieStore,
		Indexer:       indexer,
		Gateway:       gateway,
		State:         genesis,
		GenesisTime:   genesis.GenesisTime,
		EmailPassword: emailpassword,
		Port:          3000,
		Path:          path,
	}

	attorney, finalize := api.NewGeneralAttorneyServer(config)
	if attorney == nil {
		finalize <- errors.New("could not create attorney server")
		return finalize
	}
	network.LaunchProxy("localhost:6000", "localhost:5100", axeNodeToken, gatewayToken, credentials, gateway, attorney)
	return finalize
}
