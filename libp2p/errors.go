package libp2p

import "errors"

var (
	ErrNotPeerAddrInfo         = errors.New("not peer.AddrInfo")
	ErrRequirePeerID           = errors.New("Libp2pEndpoint requires peer.ID")
	ErrRequireProtoKadMessage  = errors.New("Libp2pEndpoint requires ProtoKadMessage")
	ErrRequireProtoKadResponse = errors.New("Libp2pEndpoint requires ProtoKadResponseMessage")
)
