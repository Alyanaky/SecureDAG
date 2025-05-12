package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p-kad-dht"
)

func NewDHT(ctx context.Context) (*dht.IpfsDHT, host.Host, error) {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
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
