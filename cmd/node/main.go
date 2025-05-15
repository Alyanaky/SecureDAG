package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

func main() {
	ctx := context.Background()
	
	storePath := filepath.Join(".", "storage", fmt.Sprintf("node-%d", time.Now().UnixNano()))
	storage, err := storage.NewBadgerStore(storePath)
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	kadDHT, node, err := p2p.NewDHT(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer kadDHT.Close()

	node.SetStreamHandler("/secure-dag/1.0", func(s network.Stream) {
		defer s.Close()
		buf := make([]byte, 1024)
		n, _ := s.Read(buf)
		msg := string(buf[:n])
		
		switch {
		case strings.HasPrefix(msg, "GET:"):
			hash := strings.TrimPrefix(msg, "GET:")
			data, err := storage.GetBlock(hash)
			if err == nil {
				s.Write(data)
			}
		case strings.HasPrefix(msg, "DAG:"):
			hash := strings.TrimPrefix(msg, "DAG:")
			data, _ := storage.GetBlock(hash)
			s.Write(data)
		}
	})

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "put":
			content := []byte("SecureDAG data")
			hashes, _ := storage.SplitAndStore(bytes.NewReader(content), 256)
			fmt.Printf("Stored hashes: %v\n", hashes)
			for _, hash := range hashes {
				data, _ := storage.GetBlock(hash)
				p2p.ReplicateBlock(ctx, kadDHT, hash, data, 3)
			}

		case "get":
			hash := os.Args[2]
			data, _ := p2p.GetFromDHT(ctx, kadDHT, hash)
			fmt.Printf("Retrieved data: %s\n", string(data))

		case "keys":
			fmt.Printf("Public Key:\n%s\n", crypto.PublicKeyToBytes(storage.PublicKey()))

		case "put-encrypted":
			content, _ := os.ReadFile(os.Args[2])
			hashes, _ := storage.SplitAndStore(bytes.NewReader(content), 256)
			for _, hash := range hashes {
				data, _ := storage.GetBlock(hash)
				p2p.ReplicateBlock(ctx, kadDHT, hash, data, 3)
			}

		case "put-dag":
			content, _ := os.ReadFile(os.Args[2])
			hashes, _ := storage.SplitAndStore(bytes.NewReader(content), 256)
			root, _ := storage.BuildDAG(hashes)
			dagHash, _ := storage.StoreDAG(root)
			p2p.ReplicateBlock(ctx, kadDHT, dagHash, nil, 3)
			fmt.Printf("DAG root: %s\n", dagHash)

		case "verify-dag":
			rootData, _ := p2p.GetFromDHT(ctx, kadDHT, os.Args[2])
			var root dag.Node
			json.Unmarshal(rootData, &root)
			if err := dag.Verify(&root); err != nil {
				fmt.Printf("Verification failed: %v\n", err)
			} else {
				fmt.Println("DAG is valid")
			}

		default:
			log.Fatal("Unknown command")
		}
	}

	select {}
}
