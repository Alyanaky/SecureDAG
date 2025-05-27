package p2p

import (
    "context"
    "log"
    dht "github.com/libp2p/go-libp2p-kad-dht"
    "github.com/libp2p/go-libp2p/core/peer"
)

type DHTOperations struct {
    dht *dht.IpfsDHT
}

func NewDHTOperations(dht *dht.IpfsDHT) *DHTOperations {
    return &DHTOperations{dht: dht}
}

func (ops *DHTOperations) ReplicateData(ctx context.Context, key string, data []byte) error {
    err := ops.dht.PutValue(ctx, key, data)
    if err != nil {
        return err
    }

    providers, err := ops.dht.FindProviders(ctx, key)
    if err != nil {
        return err
    }

    if len(providers) < 3 {
        log.Printf("Warning: only %d providers found for key %s", len(providers), key)
    }

    return nil
}
