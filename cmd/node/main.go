package main

import (
	"context"
	"fmt"
	"log"
	"os"
	
	"github.com/Alyanaky/SecureDAG/internal/p2p"
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

	fmt.Printf("Node ID: %s\n", node.ID())
	fmt.Printf("Addresses:\n")
	for _, addr := range node.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr, node.ID())
	}

	node.SetStreamHandler("/secure-dag/1.0", func(s network.Stream) {
		defer s.Close()
		buf := make([]byte, 1024)
		n, _ := s.Read(buf)
		fmt.Printf("Received: %s\n", string(buf[:n]))
		s.Write([]byte("ACK: " + string(buf[:n])))
	})

	if len(os.Args) > 1 {
		peerAddr := os.Args[1]
		addr, _ := ma.NewMultiaddr(peerAddr)
		peerInfo, _ := peer.AddrInfoFromP2pAddr(addr)
		
		if err := node.Connect(ctx, *peerInfo); err != nil {
			log.Fatal(err)
		}
		
		s, _ := node.NewStream(ctx, peerInfo.ID, "/secure-dag/1.0")
		s.Write([]byte("Hello from node 2"))
		defer s.Close()
	}

	select {}
}
