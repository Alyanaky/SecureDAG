package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/crypto"
	"github.com/Alyanaky/SecureDAG/internal/dag"
	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/Alyanaky/SecureDAG/internal/sync"
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

func main() {
	ctx := context.Background()

	storePath := filepath.Join(".", "storage", fmt.Sprintf("node-%d", time.Now().UnixNano()))
	store, err := storage.NewBadgerStore(storePath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	kadDHT, node, err := p2p.NewDHT(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer kadDHT.Close()

	gossip := sync.NewGossipManager(node)
	go gossip.Start(ctx)

	node.SetStreamHandler("/secure-dag/1.0", func(s network.Stream) {
		defer s.Close()
		buf := make([]byte, 1024)
		n, _ := s.Read(buf)
		msg := string(buf[:n])

		switch {
		case strings.HasPrefix(msg, "GET:"):
			handleGetRequest(s, msg, store)
		case strings.HasPrefix(msg, "DAG:"):
			handleDAGRequest(s, msg, store)
		}
	})

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "api":
			StartAPIServer(store)
		case "put", "get", "keys", "put-encrypted", "put-dag", "verify-dag":
			handleCLICommand(ctx, os.Args, kadDHT, store)
		default:
			log.Fatal("Unknown command")
		}
	}

	select {}
}

func handleGetRequest(s network.Stream, msg string, store *storage.BadgerStore) {
	hash := strings.TrimPrefix(msg, "GET:")
	if data, err := store.GetBlock(hash); err == nil {
		s.Write(data)
	}
}

func handleDAGRequest(s network.Stream, msg string, store *storage.BadgerStore) {
	hash := strings.TrimPrefix(msg, "DAG:")
	if data, err := store.GetBlock(hash); err == nil {
		s.Write(data)
	}
}

func handleCLICommand(ctx context.Context, args []string, dht *dht.IpfsDHT, store *storage.BadgerStore) {
	switch args[1] {
	case "put":
		content := []byte("SecureDAG data")
		hashes, _ := store.SplitAndStore(bytes.NewReader(content), 256)
		fmt.Printf("Stored hashes: %v\n", hashes)
		for _, hash := range hashes {
			data, _ := store.GetBlock(hash)
			p2p.ReplicateBlock(ctx, dht, hash, data, 3)
		}

	case "get":
		hash := args[2]
		data, _ := p2p.GetFromDHT(ctx, dht, hash)
		fmt.Printf("Retrieved data: %s\n", string(data))

	case "keys":
		fmt.Printf("Public Key:\n%s\n", crypto.PublicKeyToBytes(store.PublicKey()))

	case "put-encrypted":
		content, _ := os.ReadFile(args[2])
		hashes, _ := store.SplitAndStore(bytes.NewReader(content), 256)
		for _, hash := range hashes {
			data, _ := store.GetBlock(hash)
			p2p.ReplicateBlock(ctx, dht, hash, data, 3)
		}

	case "put-dag":
		content, _ := os.ReadFile(args[2])
		hashes, _ := store.SplitAndStore(bytes.NewReader(content), 256)
		root, _ := store.BuildDAG(hashes)
		dagHash, _ := store.StoreDAG(root)
		p2p.ReplicateBlock(ctx, dht, dagHash, nil, 3)
		fmt.Printf("DAG root: %s\n", dagHash)

	case "verify-dag":
		rootData, _ := p2p.GetFromDHT(ctx, dht, args[2])
		var root dag.Node
		proto.Unmarshal(rootData, &root)
		if err := dag.Verify(&root); err != nil {
			fmt.Printf("Verification failed: %v\n", err)
		} else {
			fmt.Println("DAG is valid")
		}
	}
}
