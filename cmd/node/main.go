package main

import (
	"context"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
)

func main() {
	ctx := context.Background()
	node, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		log.Fatal(err)
	}

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

	<-ctx.Done()
}
