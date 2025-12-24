package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/armon/go-socks5"
	"golang.org/x/crypto/ssh"
)

func main() {
	// 1. Define and parse arguments
	flag.Parse()
	if flag.NArg() < 3 {
		fmt.Println("Usage: proxy-client <private-key-path> <ssh-server-addr> <proxy-listen-addr>")
		fmt.Println("Example: ./proxy-client ~/.ssh/id_rsa localhost:22 127.0.0.1:3080")
		os.Exit(1)
	}

	keyPath := flag.Arg(0)
	sshAddr := flag.Arg(1)
	proxyAddr := flag.Arg(2)

	// 2. Read and parse the private key
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	// 3. Configure the SSH Client (User hardcoded to root as requested)
	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Use a secure callback for production
	}

	// 4. Connect to the SSH server
	client, err := ssh.Dial("tcp", sshAddr, config)
	if err != nil {
		log.Fatalf("unable to connect to SSH server: %v", err)
	}
	defer client.Close()
	log.Printf("Connected to SSH server at %s", sshAddr)

	// 5. Create a SOCKS5 server using the SSH client dialer
	conf := &socks5.Config{
		// Add 'ctx context.Context' as the first parameter
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return client.Dial(network, addr)
		},
	}

	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("failed to create socks5 server: %v", err)
	}

	// 6. Start the local SOCKS5 listener
	log.Printf("Starting SOCKS5 proxy on %s", proxyAddr)
	if err := server.ListenAndServe("tcp", proxyAddr); err != nil {
		log.Fatalf("failed to listen on %s: %v", proxyAddr, err)
	}
}
