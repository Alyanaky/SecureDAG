package sync

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/storage"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type BlockAlert struct {
	BlockHash string `json:"hash"`
	Type      string `json:"type"`
	Timestamp int64  `json:"ts"`
}

type GossipManager struct {
	host      host.Host
	peers     map[peer.ID]time.Time
	broadcast chan []byte
	storage   *storage.BadgerStore
}

func NewGossipManager(h host.Host, store *storage.BadgerStore) *GossipManager {
	return &GossipManager{
		host:      h,
		peers:     make(map[peer.ID]time.Time),
		broadcast: make(chan []byte, 100),
		storage:   store,
	}
}

func (g *GossipManager) Start(ctx context.Context) {
	go g.peerDiscovery(ctx)
	go g.messageBroadcast(ctx)
	go g.messageHandler(ctx)
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

func (g *GossipManager) messageHandler(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			g.host.SetStreamHandler("/secure-dag/gossip/1.0", func(s network.Stream) {
				defer s.Close()
				buf := make([]byte, 1024)
				n, _ := s.Read(buf)
				g.HandleGossipMessage(buf[:n])
			})
		}
	}
}

func (g *GossipManager) HandleGossipMessage(msg []byte) {
	var alert BlockAlert
	if json.Unmarshal(msg, &alert) == nil {
		if alert.Type == "REPLICA_ALERT" {
			g.processReplicaAlert(alert)
		}
	}
}

func (g *GossipManager) processReplicaAlert(alert BlockAlert) {
	if g.storage.BlockExists(alert.BlockHash) {
		g.storage.IncreaseReplicaCount(alert.BlockHash)
	}
}

func (g *GossipManager) BroadcastReplicaAlert(hash string) {
	alert := BlockAlert{
		BlockHash: hash,
		Type:      "REPLICA_ALERT",
		Timestamp: time.Now().Unix(),
	}
	msg, _ := json.Marshal(alert)
	g.broadcast <- msg
}
