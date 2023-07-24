package addrinfo

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
)

type AddrInfo struct {
	peer.AddrInfo
	id *peerid.PeerID
}

var _ address.NodeAddr[key.Key256] = (*AddrInfo)(nil)

func NewAddrInfo(ai peer.AddrInfo) *AddrInfo {
	return &AddrInfo{
		AddrInfo: ai,
		id:       peerid.NewPeerID(ai.ID),
	}
}

func (ai AddrInfo) Key() key.Key256 {
	return ai.id.Key()
}

func (ai AddrInfo) String() string {
	return ai.id.String()
}

func (ai AddrInfo) PeerID() *peerid.PeerID {
	return ai.id
}

func (ai AddrInfo) NodeID() address.NodeID[key.Key256] {
	return ai.id
}

func (ai AddrInfo) Addresses() []address.Addr {
	addrs := make([]address.Addr, len(ai.Addrs))
	for i, a := range ai.Addrs {
		addrs[i] = a
	}
	return addrs
}
