package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/p2p"
	"github.com/Alyanaky/SecureDAG/internal/storage"
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

	store.StartSelfHeal(ctx, kadDHT)

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

	select {}
}
