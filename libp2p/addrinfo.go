package libp2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
)

type AddrInfo struct {
	peer.AddrInfo
	id *PeerID
}

var _ kad.NodeInfo[key.Key256, multiaddr.Multiaddr] = (*AddrInfo)(nil)

func NewAddrInfo(ai peer.AddrInfo) *AddrInfo {
	return &AddrInfo{
		AddrInfo: ai,
		id:       NewPeerID(ai.ID),
	}
}

func (ai AddrInfo) Key() key.Key256 {
	return ai.id.Key()
}

func (ai AddrInfo) String() string {
	return ai.id.String()
}

func (ai AddrInfo) PeerID() *PeerID {
	return ai.id
}

func (ai AddrInfo) ID() kad.NodeID[key.Key256] {
	return ai.id
}

func (ai AddrInfo) Addresses() []multiaddr.Multiaddr {
	addrs := make([]multiaddr.Multiaddr, len(ai.Addrs))
	copy(addrs, ai.Addrs)
	return addrs
}
