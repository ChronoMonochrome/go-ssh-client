package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/armon/go-socks5"
	"golang.org/x/crypto/ssh"
)

// keepAlive sends a heartbeat signal to the server to keep the connection open.
func keepAlive(client *ssh.Client, interval time.Duration, maxRetries int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	retries := 0
	for range ticker.C {
		// Send a global "keepalive" request.
		// "keepalive@golang.org" is a standard name used in Go's SSH library.
		_, _, err := client.SendRequest("keepalive@golang.org", true, nil)
		if err != nil {
			retries++
			log.Printf("Keepalive failed (%d/%d): %v", retries, maxRetries, err)
			if retries >= maxRetries {
				log.Printf("Max keepalive retries reached. Closing connection.")
				client.Close()
				return
			}
		} else {
			retries = 0 // Reset retries on success
		}
	}
}

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
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		// Timeout for the initial connection handshake
		Timeout: 30 * time.Second,
	}

	// 4. Connect to the SSH server
	client, err := ssh.Dial("tcp", sshAddr, config)
	if err != nil {
		log.Fatalf("unable to connect to SSH server: %v", err)
	}
	defer client.Close()
	log.Printf("Connected to SSH server at %s", sshAddr)

	// START KEEPALIVE GOROUTINE
	// Interval: 60s (ClientAliveInterval)
	// MaxRetries: 1000 (ClientAliveCountMax)
	go keepAlive(client, 60*time.Second, 1000)

	log.Printf("Connected to %s with keepalives enabled.", sshAddr)

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
