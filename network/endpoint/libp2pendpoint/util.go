package libp2pendpoint

import (
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/addrinfo"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-msgio/pbio"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func WriteMsg(s network.Stream, msg protoreflect.ProtoMessage) error {
	w := pbio.NewDelimitedWriter(s)
	return w.WriteMsg(msg)
}

func ReadMsg(s network.Stream, msg protoreflect.ProtoMessage) error {
	r := pbio.NewDelimitedReader(s, network.MessageSizeMax)
	return r.ReadMsg(msg)
}

func PeerInfo(e endpoint.Endpoint, p address.NodeID) (peer.AddrInfo, error) {
	netAddr, err := e.NetworkAddress(p)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	ai, ok := netAddr.(addrinfo.AddrInfo)
	if !ok {
		return peer.AddrInfo{}, ErrNotPeerAddrInfo
	}
	return ai.AddrInfo, nil
}
