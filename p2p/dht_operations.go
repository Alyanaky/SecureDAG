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

func GetFromDHT(ctx context.Context, dht *dht.IpfsDHT, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, getTimeout)
	defer cancel()
	
	return dht.GetValue(ctx, "/secure-dag/"+key)
}
