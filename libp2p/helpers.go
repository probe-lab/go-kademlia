package libp2p

import (
	"bytes"
	"errors"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/endpoint"
)

var ErrNoValidAddresses = errors.New("no valid addresses")

func FindPeerRequest(p *PeerID) *Message {
	marshalledPeerid, _ := p.MarshalBinary()
	return &Message{
		Type: Message_FIND_NODE,
		Key:  marshalledPeerid,
	}
}

func FindPeerResponse(peers []kad.NodeID[key.Key256], e endpoint.NetworkedEndpoint[key.Key256, PeerID, multiaddr.Multiaddr]) *Message {
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
	return PeerID{ID: p}.Key()
}

type FindNodeMessage struct {
	msg *Message
}

func (f *FindNodeMessage) CloserNodes() []kad.NodeInfo[key.Key256, PeerID, multiaddr.Multiaddr] {
	closerPeers := f.msg.GetCloserPeers()
	if closerPeers == nil {
		return []kad.NodeInfo[key.Key256, PeerID, multiaddr.Multiaddr]{}
	}
	return ParsePeers(closerPeers)
}

func (f *FindNodeMessage) Records() []PeerRecord {
	for _, cp := range f.msg.GetCloserPeers() {
		if bytes.Equal(f.msg.Key, cp.Id) {
			a, err := PBPeerToPeerInfo(cp)
			if err != nil {
				continue
			}
			return []PeerRecord{{
				AddrInfo: a.AddrInfo,
			}}
		}
	}
	return []PeerRecord{}
}

type GetProvidersMessage struct {
	msg *Message
}

func (g *GetProvidersMessage) CloserNodes() []kad.NodeInfo[key.Key256, PeerID, multiaddr.Multiaddr] {
	closerPeers := g.msg.GetCloserPeers()
	if closerPeers == nil {
		return []kad.NodeInfo[key.Key256, PeerID, multiaddr.Multiaddr]{}
	}
	return ParsePeers(closerPeers)
}

func (g *GetProvidersMessage) Records() []PeerRecord {
	var providers []PeerRecord
	for _, pp := range g.msg.GetProviderPeers() {
		a, err := PBPeerToPeerInfo(pp)
		if err != nil {
			continue
		}
		providers = append(providers, PeerRecord{AddrInfo: a.AddrInfo})
	}
	return providers
}

func PBPeerToPeerInfo(pbp *Message_Peer) (*AddrInfo, error) {
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

	return NewAddrInfo(peer.AddrInfo{
		ID:    peer.ID(pbp.Id),
		Addrs: addrs,
	}), nil
}

func ParsePeers(pbps []*Message_Peer) []kad.NodeInfo[key.Key256, PeerID, multiaddr.Multiaddr] {
	peers := make([]kad.NodeInfo[key.Key256, PeerID, multiaddr.Multiaddr], 0, len(pbps))
	for _, p := range pbps {
		pi, err := PBPeerToPeerInfo(p)
		if err == nil {
			peers = append(peers, pi)
		}
	}
	return peers
}

func NodeIDsToPbPeers(peers []kad.NodeID[key.Key256], e endpoint.NetworkedEndpoint[key.Key256, PeerID, multiaddr.Multiaddr]) []*Message_Peer {
	if len(peers) == 0 || e == nil {
		return nil
	}

	pbPeers := make([]*Message_Peer, 0, len(peers))
	for _, n := range peers {
		p := n.(*PeerID)

		id, err := e.NetworkAddress(n)
		if err != nil {
			continue
		}
		// convert NetworkAddress to []multiaddr.Multiaddr
		addrs := id.(*AddrInfo).Addrs
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
