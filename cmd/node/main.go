package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/crypto"
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
		
		if strings.HasPrefix(msg, "GET:") {
			hash := strings.TrimPrefix(msg, "GET:")
			data, err := storage.GetBlock(hash)
			if err == nil {
				s.Write(data)
			}
		}
	})

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "put":
			content := []byte("Persistent SecureDAG data!")
			hashes, _ := storage.SplitAndStore(bytes.NewReader(content), 256)
			fmt.Printf("Stored hashes: %v\n", hashes)
			
			for _, hash := range hashes {
				if data, err := storage.GetBlock(hash); err == nil {
					p2p.ReplicateBlock(ctx, kadDHT, hash, data, 3)
					fmt.Printf("Published hash %s to DHT\n", hash)
				}
			}
			
		case "get":
			if len(os.Args) < 3 {
				log.Fatal("Usage: get <hash>")
			}
			hash := os.Args[2]
			data, _ := p2p.GetFromDHT(ctx, kadDHT, hash)
			fmt.Printf("Retrieved data: %s\n", string(data))
			
		case "keys":
			publicKeyPEM := crypto.PublicKeyToBytes(storage.PublicKey())
			fmt.Printf("Public Key:\n%s\n", publicKeyPEM)
			
		case "put-encrypted":
			if len(os.Args) < 4 {
				log.Fatal("Usage: put-encrypted <file> <public-key.pem>")
			}
			
			pubKeyBytes, _ := os.ReadFile(os.Args[3])
			recipientPubKey, _ := crypto.BytesToPublicKey(pubKeyBytes)
			
			content, _ := os.ReadFile(os.Args[2])
			hashes, _ := storage.SplitAndStore(bytes.NewReader(content), 256)
			
			for _, hash := range hashes {
				data, _ := storage.GetBlock(hash)
				p2p.ReplicateBlock(ctx, kadDHT, hash, data, 3)
			}
			fmt.Printf("Encrypted hashes: %v\n", hashes)
		}
	}

	select {}
}
