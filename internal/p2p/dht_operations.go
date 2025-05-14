package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p-kad-dht"
)

const (
	putTimeout = 10 * time.Second
	getTimeout = 15 * time.Second
)

func PutToDHT(ctx context.Context, dht *dht.IpfsDHT, key string, value []byte) error {
	ctx, cancel := context.WithTimeout(ctx, putTimeout)
	defer cancel()
	
	return dht.PutValue(ctx, "/secure-dag/"+key, value)
}

func ReplicateBlock(ctx context.Context, dht *dht.IpfsDHT, key string, value []byte, replicas int) error {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(replicas)*putTimeout)
	defer cancel()

	providers := make([]peer.ID, 0, replicas)
	
	// Находим N ближайших узлов
	peers, err := dht.FindPeers(ctx, key)
	if err != nil {
		return err
	}
	
	for p := range peers {
		if len(providers) >= replicas {
			break
		}
		if p.ID != dht.Host().ID() { // Исключаем себя
			providers = append(providers, p.ID)
		}
	}
	
	// Сохраняем на выбранные узлы
	for _, pid := range providers {
		err := dht.PutValue(ctx, "/secure-dag/"+key, value, dht.Quorum(1))
		if err != nil {
			log.Printf("Failed to replicate to %s: %v", pid, err)
		}
	}
	
	return nil
}

func GetFromDHT(ctx context.Context, dht *dht.IpfsDHT, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, getTimeout)
	defer cancel()
	
	return dht.GetValue(ctx, "/secure-dag/"+key)
}
