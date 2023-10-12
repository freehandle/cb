package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/freehandle/cb/vault"
	"golang.org/x/crypto/ssh/terminal"
)

var usage = `Usage: 

	safe <path-to-vault-file> <command> [arguments]

The commands are:

	new       create new random key pair and secure them on vault
   	balance   show balance of tokens associated to the key on breeze network
	config    global configuration for safe command
   	deposit   deposit token balance for a key on breeze network 
	grant     grant power of attorney to another key on axe protocol
	list      list all know tokens 
	revoke    revoke power of attorney to another key on axe protocol
	send      transfer token to another key
	withdraw  withdraw token associated to key on breeze network
	

Use "safe help <command>" for more information about a command.

`

func ParseCommand(params ...string) {
	if len(params) == 0 {
		return
	}
	if params[0] == "help" {
		if len(params) > 1 {
			if params[1] == "new" {
				fmt.Print(helpNew)
				os.Exit(0)
			}
		}
	}
}

func main() {
	ParseCommand(os.Args[1:]...)
	if len(os.Args) < 2 {
		fmt.Println(usage)
		return
	}
	var safe *vault.SecureVault
	if stat, _ := os.Stat(os.Args[1]); stat == nil {
		fmt.Print("File does not exist. Create new [yes/no]?")
		var yes string
		fmt.Scan(&yes)
		yes = strings.TrimSpace(strings.ToLower(yes))
		if yes == "yes" || yes == "y" {
			fmt.Println("Enter pass phrase to secure safe vault:")
			password, err := terminal.ReadPassword(0)
			if err != nil {
				fmt.Printf("Error reading password: %v\n", err)
				return
			}
			safe = vault.NewSecureVault([]byte(password), os.Args[1])
			if safe == nil {
				fmt.Println("Could not create vault")
				return
			}
		} else {
			return
		}
	} else if stat.IsDir() {
		fmt.Println("File is a directory")
		return
	} else {
		fmt.Println("Enter pass phrase:")
		password, err := terminal.ReadPassword(0)
		if err != nil {
			fmt.Printf("Error reading password: %v\n", err)
			return
		}
		safe = vault.OpenVaultFromPassword([]byte(password), os.Args[1])
		if safe == nil {
			fmt.Println("Could not open vault")
			return
		}
	}
}
