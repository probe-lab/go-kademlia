package peerid

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/plprobelab/go-kademlia/key"
	builder "github.com/plprobelab/go-kademlia/key/sha256key256"
	"github.com/plprobelab/go-kademlia/network/address"
)

// PeerID is a libp2p Peer ID representation of a node identifier.
type PeerID struct {
	peer.ID
}

var _ address.NodeID = (*PeerID)(nil)

func NewPeerID(p peer.ID) *PeerID {
	return &PeerID{p}
}

func (id *PeerID) Key() key.KadKey {
	return builder.StringKadID(string(id.ID))
}

func (id *PeerID) NodeID() address.NodeID {
	return id
}

func (id *PeerID) Equal(other address.NodeID) bool {
	tother, ok := other.(*PeerID)
	if !ok {
		return false
	}
	if id == nil || tother == nil {
		return false
	}
	return id.ID == tother.ID
}
