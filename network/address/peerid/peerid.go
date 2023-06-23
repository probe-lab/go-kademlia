package peerid

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/libp2p/go-libp2p-kad-dht/key"
	builder "github.com/libp2p/go-libp2p-kad-dht/key/sha256key256"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
)

type PeerID struct {
	peer.ID
}

var _ address.NodeID = (*PeerID)(nil)

func NewPeerID(p peer.ID) *PeerID {
	return &PeerID{p}
}

func (id PeerID) Key() key.KadKey {
	return builder.StringKadID(string(id.ID))
}

func (id PeerID) NodeID() address.NodeID {
	return &id
}
