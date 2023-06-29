package ipfsv1

import (
	"errors"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/addrinfo"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var (
	ErrNoValidAddresses = errors.New("no valid addresses")
)

var _ message.ProtoKadRequestMessage = (*Message)(nil)
var _ message.ProtoKadResponseMessage = (*Message)(nil)

func FindPeerRequest(p *peerid.PeerID) *Message {
	marshalledPeerid, _ := p.MarshalBinary()
	return &Message{
		Type: Message_FIND_NODE,
		Key:  marshalledPeerid,
	}
}

func FindPeerResponse(peers []address.NodeID, e endpoint.NetworkedEndpoint) *Message {
	return &Message{
		Type:        Message_FIND_NODE,
		CloserPeers: NodeIDsToPbPeers(peers, e),
	}
}

func (msg *Message) Target() key.KadKey {
	p, err := peer.IDFromBytes(msg.GetKey())
	if err != nil {
		return nil
	}
	return peerid.PeerID{ID: p}.Key()
}

func (msg *Message) EmptyResponse() message.MinKadResponseMessage {
	return &Message{}
}

func (msg *Message) CloserNodes() []address.NodeID {
	closerPeers := msg.GetCloserPeers()
	if closerPeers == nil {
		return []address.NodeID{}
	}
	return ParsePeers(closerPeers)
}

func PBPeerToPeerInfo(pbp *Message_Peer) (*addrinfo.AddrInfo, error) {
	addrs := make([]multiaddr.Multiaddr, 0, len(pbp.Addrs))
	for _, a := range pbp.Addrs {
		addr, err := multiaddr.NewMultiaddrBytes(a)
		if err == nil {
			addrs = append(addrs, addr)
		}
	}
	if len(addrs) == 0 {
		return nil, ErrNoValidAddresses
	}

	return addrinfo.NewAddrInfo(peer.AddrInfo{
		ID:    peer.ID(pbp.Id),
		Addrs: addrs,
	}), nil
}

func ParsePeers(pbps []*Message_Peer) []address.NodeID {
	peers := make([]address.NodeID, 0, len(pbps))
	for _, p := range pbps {
		pi, err := PBPeerToPeerInfo(p)
		if err == nil {
			peers = append(peers, pi)
		}
	}
	return peers
}

func NodeIDsToPbPeers(peers []address.NodeID, e endpoint.NetworkedEndpoint) []*Message_Peer {
	if len(peers) == 0 || e == nil {
		return nil
	}

	pbPeers := make([]*Message_Peer, 0, len(peers))
	for _, n := range peers {
		p := n.(*peerid.PeerID)

		id, err := e.NetworkAddress(n)
		if err != nil {
			continue
		}
		// convert NetworkAddress to []multiaddr.Multiaddr
		addrs := id.(*addrinfo.AddrInfo).Addrs
		pbAddrs := make([][]byte, len(addrs))
		// convert multiaddresses to bytes
		for i, a := range addrs {
			pbAddrs[i] = a.Bytes()
		}

		connectedness, err := e.Connectedness(n)
		if err != nil {
			continue
		}
		pbPeers = append(pbPeers, &Message_Peer{
			Id:         []byte(p.ID),
			Addrs:      pbAddrs,
			Connection: Message_ConnectionType(connectedness),
		})
	}
	return pbPeers
}
