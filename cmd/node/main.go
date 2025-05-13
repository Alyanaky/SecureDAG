package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
)

func main() {
	ctx := context.Background()
	
	kadDHT, node, err := p2p.NewDHT(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer kadDHT.Close()

	storage := storage.NewStorage()

	node.SetStreamHandler("/secure-dag/1.0", func(s network.Stream) {
		defer s.Close()
		buf := make([]byte, 1024)
		n, _ := s.Read(buf)
		msg := string(buf[:n])
		
		if strings.HasPrefix(msg, "STORE:") {
			hash := strings.TrimPrefix(msg, "STORE:")
			if data, exists := storage.GetBlock(hash); exists {
				p2p.PutToDHT(ctx, kadDHT, hash, data)
			}
		}
	})

	if len(os.Args) > 1 && os.Args[1] == "put" {
		content := []byte("Hello SecureDAG!")
		hashes, _ := storage.SplitAndStore(bytes.NewReader(content), 256)
		fmt.Printf("Stored hashes: %v\n", hashes)
		
		for _, hash := range hashes {
			if data, exists := storage.GetBlock(hash); exists {
				p2p.PutToDHT(ctx, kadDHT, hash, data)
				fmt.Printf("Published hash %s to DHT\n", hash)
			}
		}
	}

	if len(os.Args) > 1 && os.Args[1] == "get" {
		hash := os.Args[2]
		data, _ := p2p.GetFromDHT(ctx, kadDHT, hash)
		fmt.Printf("Retrieved data: %s\n", string(data))
	}

	select {}
}
