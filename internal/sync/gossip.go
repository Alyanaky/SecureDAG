package sync

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type GossipManager struct {
	host      host.Host
	peers     map[peer.ID]time.Time
	broadcast chan []byte
}

func NewGossipManager(h host.Host) *GossipManager {
	return &GossipManager{
		host:      h,
		peers:     make(map[peer.ID]time.Time),
		broadcast: make(chan []byte, 100),
	}
}

func (g *GossipManager) Start(ctx context.Context) {
	go g.peerDiscovery(ctx)
	go g.messageBroadcast(ctx)
}

func (g *GossipManager) peerDiscovery(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			for _, p := range g.host.Network().Peers() {
				g.peers[p] = time.Now()
			}
		case <-ctx.Done():
			return
		}
	}
}

func (g *GossipManager) messageBroadcast(ctx context.Context) {
	for {
		select {
		case msg := <-g.broadcast:
			for pid := range g.peers {
				s, err := g.host.NewStream(ctx, pid, "/secure-dag/gossip/1.0")
				if err != nil {
					continue
				}
				s.Write(msg)
				s.Close()
			}
		case <-ctx.Done():
			return
		}
	}
}
