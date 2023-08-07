package libp2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	mh "github.com/multiformats/go-multihash"
	mhreg "github.com/multiformats/go-multihash/core"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type PeerID struct {
	peer.ID
}

var _ kad.NodeID[key.Key256] = (*PeerID)(nil)

func NewPeerID(p peer.ID) PeerID {
	return PeerID{p}
}

func (id PeerID) Key() key.Key256 {
	hasher, _ := mhreg.GetHasher(mh.SHA2_256)
	hasher.Write([]byte(id.ID))
	return key.NewKey256(hasher.Sum(nil))
}

func (id PeerID) NodeID() kad.NodeID[key.Key256] {
	return &id
}
