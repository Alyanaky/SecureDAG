package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
)

func NewDHT(ctx context.Context) (*dht.IpfsDHT, host.Host, error) {
	h, err := libp2p.New()
	if err != nil {
		return nil, nil, err
	}

	kadDHT, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, nil, err
	}

	if err = kadDHT.Bootstrap(ctx); err != nil {
		return nil, nil, err
	}

	return kadDHT, h, nil
}
