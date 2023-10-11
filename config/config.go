package util

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/freehandle/breeze/crypto"
	"github.com/freehandle/cb/vault"
	"golang.org/x/term"
)

func vaultExists(vaultpath string) bool {
	info, err := os.Stat(vaultpath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func GetOrSetCredentialsFromVault(vaultpath string) (crypto.PrivateKey, string) {
	if vaultExists(vaultpath) {
		fmt.Print("Enter passphrase of secure vault:")
		bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
		secrets := vault.OpenVaultFromPassword(bytePassword, vaultpath)
		secrets.Close()
		credentials := secrets.SecretKey
		return credentials, ""
	} else {
		fmt.Print("Enter passphrase for a new secure vault:")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			log.Fatalf("could not read password: %v", err)
		}
		secrets := vault.NewSecureVault(bytePassword, vaultpath)
		return secrets.SecretKey, fmt.Sprintf("Token %v generated for vault %v\nUpdate your config file.\n", secrets.SecretKey.PublicKey(), vaultpath)
	}
}

func ReadConfigFile(configpath string, config any) {
	data, err := os.ReadFile(configpath)
	if err != nil {
		log.Fatalf("could not read config file: %v\n", err)
	}
	if err := json.Unmarshal(data, config); err != nil {
		log.Fatalf("could not read config file: %v\n", err)
	}
}

func ShutdownEvents() chan struct{} {
	c := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		<-c
		log.Print("shuting down")
		done <- struct{}{}
	}()
	return done
}

func AskConfirm(text string) bool {
	var s string
	fmt.Printf("%v (Y/n): ", text)
	_, err := fmt.Scan(&s)
	if err != nil {
		panic(err)
	}

	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	if s == "y" || s == "yes" {
		return true
	}
	return false
}

func DefaultHomeDir() string {
	env := "HOME"
	if runtime.GOOS == "windows" {
		env = "USERPROFILE"
	} else if runtime.GOOS == "plan9" {
		env = "home"
	}
	if home := os.Getenv(env); home != "" {
		return home
	}
	log.Fatalf("could not find home directory")
	return ""
}

func CreateFolderIfNotExists(folder string) bool {
	if _, err := os.ReadDir(folder); err == nil {
		return true
	}
	if err := os.Mkdir(folder, fs.ModePerm); err != nil {
		log.Fatalf("could not create folder on home directory: %v", err)
	}
	return false
}

func CreateFileIfNotExists(folder, fileName string) (*os.File, bool) {
	path := DefaultHomeDir()
	if folder != "" {
		path = filepath.Join(path, folder)
		CreateFolderIfNotExists(path)
	}
	path = filepath.Join(path, fileName)
	file, err := os.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err == nil {
		return file, true
	}
	file, err = os.Create(path)
	if err != nil {
		log.Fatalf("could not create file: %v", path)
	}
	return file, false
}

func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func OpenVault(path string) *vault.SecureVault {
	fmt.Printf("secret password: ")
	passwd, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil {
		log.Fatalf("could not read password: %v", err)
	}
	var opened *vault.SecureVault
	if FileExists(path) {
		opened = vault.OpenVaultFromPassword(passwd, path)
		if opened == nil {
			log.Fatal("could not open secure vault")
		}
		return opened
	}
	opened = vault.NewSecureVault(passwd, path)
	if opened == nil {
		log.Fatal("could not create secure vault")
	}
	return opened
}
