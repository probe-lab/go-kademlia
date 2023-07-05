package peerid

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

type PeerID struct {
	peer.ID
}

var _ address.NodeID[key.SHA256] = (*PeerID)(nil)

func NewPeerID(p peer.ID) *PeerID {
	return &PeerID{p}
}

func (id PeerID) Key() key.SHA256 {
	return key.NewSHA256([]byte(id.ID))
}

func (id PeerID) NodeID() address.NodeID[key.SHA256] {
	return &id
}
