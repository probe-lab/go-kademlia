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

var ErrNoValidAddresses = errors.New("no valid addresses")

func FindPeerRequest(p *peerid.PeerID) *Message {
	marshalledPeerid, _ := p.MarshalBinary()
	return &Message{
		Type: Message_FIND_NODE,
		Key:  marshalledPeerid,
	}
}

func FindPeerResponse(peers []address.NodeID[key.Key256], e endpoint.NetworkedEndpoint[key.Key256]) *Message {
	return &Message{
		Type:        Message_FIND_NODE,
		CloserPeers: NodeIDsToPbPeers(peers, e),
	}
}

func (msg *Message) Target() key.Key256 {
	p, err := peer.IDFromBytes(msg.GetKey())
	if err != nil {
		return key.ZeroKey256()
	}
	return peerid.PeerID{ID: p}.Key()
}

func (msg *Message) EmptyResponse() message.MinKadResponseMessage[key.Key256] {
	return &Message{}
}

func (msg *Message) CloserNodes() []address.NodeAddr[key.Key256] {
	closerPeers := msg.GetCloserPeers()
	if closerPeers == nil {
		return []address.NodeAddr[key.Key256]{}
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

func ParsePeers(pbps []*Message_Peer) []address.NodeAddr[key.Key256] {
	peers := make([]address.NodeAddr[key.Key256], 0, len(pbps))
	for _, p := range pbps {
		pi, err := PBPeerToPeerInfo(p)
		if err == nil {
			peers = append(peers, pi)
		}
	}
	return peers
}

func NodeIDsToPbPeers(peers []address.NodeID[key.Key256], e endpoint.NetworkedEndpoint[key.Key256]) []*Message_Peer {
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
