package libp2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type ProtocolFindNode struct {
	Host host.Host
}

var _ kad.Protocol[key.Key256, PeerID, ma.Multiaddr] = ProtocolFindNode{}

func (p ProtocolFindNode) Get(ctx context.Context, to PeerID, target key.Key256) (kad.Response[key.Key256, PeerID, ma.Multiaddr], error) {
	if err := p.Host.Connect(ctx, peer.AddrInfo{ID: to.ID}); err != nil {
		return nil, err
	}

	s, err := p.Host.NewStream(ctx, to.ID, "/ipfs/kad/1.0.0")
	if err != nil {
		return nil, err
	}
	defer s.Close()

	t, err := peer.Decode("QmSKVUFAyCddg2wDUdZVCfvqG5YCwwJTWY1HRmorebXcKG")
	if err != nil {
		panic(err)
	}
	tid := NewPeerID(t)

	fmt.Println("Find peer request to:", to.ID, "for key", tid.Key().HexString())
	marshalledPeerid, _ := tid.MarshalBinary()
	req := &Message{
		Type: Message_FIND_NODE,
		Key:  marshalledPeerid,
	}
	err = WriteMsg(s, req)
	if err != nil {
		return nil, err
	}
	resp := &Message{}
	err = ReadMsg(s, resp)
	if err != nil {
		return nil, err
	}

	for _, cp := range resp.CloserPeers {
		ai, err := PBPeerToPeerInfo(cp)
		if err != nil {
			continue
		}
		for _, a := range ai.Addrs {
			p.Host.Peerstore().AddAddr(ai.PeerID().ID, a, peerstore.AddressTTL)
		}
	}

	fmt.Println("Find peer response from:", to.ID, "closer", len(resp.CloserPeers))
	return resp, nil
}

func (p ProtocolFindNode) Put(ctx context.Context, to PeerID, record []byte) error {
	return nil
}

type ProtocolProviderRecords struct {
	host host.Host
}

type ProtocolIPNS struct {
	host host.Host
}
