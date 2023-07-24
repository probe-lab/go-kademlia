package peerid

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/plprobelab/go-kademlia/key"
	builder "github.com/plprobelab/go-kademlia/key/sha256key256"
	"github.com/plprobelab/go-kademlia/network/address"
)

type PeerID struct {
	peer.ID
}

var _ address.NodeID[key.Key256] = (*PeerID)(nil)

func NewPeerID(p peer.ID) *PeerID {
	return &PeerID{p}
}

func (id PeerID) Key() key.Key256 {
	return builder.StringKadID(string(id.ID))
}

func (id PeerID) NodeID() address.NodeID[key.Key256] {
	return &id
}
