package addrinfo

import (
	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/peerid"
	"github.com/libp2p/go-libp2p/core/peer"
)

type AddrInfo struct {
	peer.AddrInfo
	id *peerid.PeerID
}

var _ address.NodeID = (*AddrInfo)(nil)

func NewAddrInfo(ai peer.AddrInfo) *AddrInfo {
	return &AddrInfo{
		AddrInfo: ai,
		id:       peerid.NewPeerID(ai.ID),
	}
}

func (ai AddrInfo) Key() key.KadKey {
	return ai.id.Key()
}

func (ai AddrInfo) String() string {
	return ai.id.String()
}

func (ai AddrInfo) PeerID() *peerid.PeerID {
	return ai.id
}
