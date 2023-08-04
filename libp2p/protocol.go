package libp2p

import (
	"context"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type FindNode struct {
	host host.Host
}

var _ kad.Protocol[key.Key256, ma.Multiaddr] = FindNode{}

func (f FindNode) Get(ctx context.Context, to kad.NodeID[key.Key256], target key.Key256) (kad.Response[key.Key256, ma.Multiaddr], error) {
	pi, _ := getPeerID(to)
	if err := f.host.Connect(ctx, peer.AddrInfo{ID: pi.ID}); err != nil {
		return nil, err
	}

	s, err := f.host.NewStream(ctx, pi.ID, nil /*TODO*/)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	req := FindPeerRequest(pi)
	err = WriteMsg(s, req)
	if err != nil {
		return nil, err
	}
	resp := &Message{}
	err = ReadMsg(s, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (f FindNode) Put(ctx context.Context, to kad.NodeID[key.Key256], record []byte) error {
}
