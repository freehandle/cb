package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// relay
// index
// validator
// block history

// Gateway Port 5100
// AxeValidator Port 6000
// AxeBlockProvider Port 6001
// Breeze Incoming 5005
// Breeze Outgoing 5006

func main() {

	environment := envs()

	// blow breeze axe safe synergy

	args := os.Args
	if len(os.Args) > 1 && args[1] == "synergy" {
		synergyErr := synergyApp(breezeGatewayPk.PublicKey(), axeNodePk.PublicKey(), synergyAppPk, synergyEphemeral, environment.EmailPassword, environment.SynergyPath) // 3000 (http)
		err := <-synergyErr
		fmt.Println(err)
		return
	}

	breezeerr := breeze(breezeNodePk) // incoming 5005 outgoing 5006
	time.Sleep(200 * time.Millisecond)
	// store, blockErr := Blocks(breezeBlocksPk, breezeNodePk.PublicKey()) // incoming 5006 why no outgoing port??
	// time.Sleep(200 * time.Millisecond)
	gatewayErr := Gateway(breezeGatewayPk, breezeNodePk.PublicKey()) // port 5100
	time.Sleep(200 * time.Millisecond)
	axenode := AxeValidator(axeNodePk, breezeNodePk.PublicKey()) // port 6000
	time.Sleep(200 * time.Millisecond)
	//axeProvider := AxeBlockProvider(axeBlocksPk, axeNodePk.PublicKey()) // port 6001
	//time.Sleep(200 * time.Millisecond)

	synergyErr := synergyApp(breezeGatewayPk.PublicKey(), axeNodePk.PublicKey(), synergyAppPk, synergyEphemeral, environment.EmailPassword, environment.SynergyPath) // 3000 (http)
	time.Sleep(200 * time.Millisecond)
	safeServer := safeServer(breezeGatewayPk.PublicKey(), axeNodePk.PublicKey(), safeAppPk, environment.SafePath) // 7100 (http)

	//go ListenAndServe(store) // block listener

	//socailListener := testListener(axeProviderCredentials.PublicKey(), axeValidatorCredentials.PublicKey())
	//time.Sleep(200 * time.Millisecond)
	//go transferspacket(pk, myCredentials.PublicKey())
	//go axetest(pk, myCredentials.PublicKey())
	select {
	//case err := <-blockErr:
	//	log.Fatalf("block store unrecovarable error: %s", err)
	case err := <-gatewayErr:
		log.Fatalf("gateway unrecovarable error: %s", err)
	case err := <-axenode:
		log.Fatalf("axe node unrecovarable error: %s", err)
	//case err := <-axeProvider:
	//	log.Fatalf("axe block provider unrecovarable error: %s", err)
	case err := <-breezeerr:
		log.Fatalf("breeze unrecovarable error: %s", err)
	case err := <-safeServer:
		log.Fatalf("safe server unrecovarable error: %s", err)
	case err := <-synergyErr:
		log.Fatalf("synergy server unrecovarable error: %s", err)
	}
	//case err := <-socailListener:
	//		log.Fatalf("social listener unrecovarable error: %s", err)

}
