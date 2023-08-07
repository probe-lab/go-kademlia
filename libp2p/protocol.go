package libp2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type PeerRecord struct {
	peer.AddrInfo
}

var _ kad.Record = (*PeerRecord)(nil)

func (p PeerRecord) Key() []byte {
	k, _ := p.ID.MarshalBinary() // TODO: pass error
	return k
}

func (p PeerRecord) Value() []byte {
	return nil
}

type ProtocolFindNode struct {
	Host host.Host
}

var _ kad.Protocol[key.Key256, PeerID, ma.Multiaddr, PeerRecord] = ProtocolFindNode{}

func (p ProtocolFindNode) Get(ctx context.Context, to PeerID, target key.Key256) (kad.Response[key.Key256, PeerID, ma.Multiaddr, PeerRecord], error) {
	if err := p.Host.Connect(ctx, peer.AddrInfo{ID: to.ID}); err != nil {
		return nil, err
	}

	s, err := p.Host.NewStream(ctx, to.ID, "/ipfs/kad/1.0.0")
	if err != nil {
		return nil, err
	}
	defer s.Close()

	fmt.Println("Find peer request to:", to.ID, "for key", target.HexString())
	req := &Message{
		Type: Message_FIND_NODE,
		Key:  target.Bytes(),
	}
	err = WriteMsg(s, req)
	if err != nil {
		return nil, err
	}
	resp := &Message{}
	err = ReadMsg(s, resp)
	if err != nil {
		return nil, err
	}

	fnm := FindNodeMessage{msg: resp}

	for _, cn := range fnm.CloserNodes() {
		for _, a := range cn.Addresses() {
			p.Host.Peerstore().AddAddr(cn.ID().ID, a, peerstore.AddressTTL)
		}
	}

	fmt.Println("Find peer response from:", to.ID, "closer", len(resp.CloserPeers))

	return &fnm, nil
}

func (p ProtocolFindNode) Put(ctx context.Context, to PeerID, record PeerRecord) error {
	return nil
}

type ProviderRecords []PeerRecord

type ProtocolProviderRecords struct {
	Host host.Host
}

var _ kad.Protocol[key.Key256, PeerID, ma.Multiaddr, PeerRecord] = ProtocolProviderRecords{}

func (p ProtocolProviderRecords) Get(ctx context.Context, to PeerID, target key.Key256) (kad.Response[key.Key256, PeerID, ma.Multiaddr, PeerRecord], error) {
	if err := p.Host.Connect(ctx, peer.AddrInfo{ID: to.ID}); err != nil {
		return nil, err
	}

	s, err := p.Host.NewStream(ctx, to.ID, "/ipfs/kad/1.0.0")
	if err != nil {
		return nil, err
	}
	defer s.Close()

	fmt.Println("Find peer request to:", to.ID, "for key", target.HexString())
	req := &Message{
		Type: Message_GET_PROVIDERS,
		Key:  target.Bytes(),
	}
	err = WriteMsg(s, req)
	if err != nil {
		return nil, err
	}
	resp := &Message{}
	err = ReadMsg(s, resp)
	if err != nil {
		return nil, err
	}

	gpm := GetProvidersMessage{msg: resp}

	for _, cn := range gpm.CloserNodes() {
		for _, a := range cn.Addresses() {
			p.Host.Peerstore().AddAddr(cn.ID().ID, a, peerstore.AddressTTL)
		}
	}

	fmt.Println("Find peer response from:", to.ID, "closer", len(resp.CloserPeers))

	return &gpm, nil
}

func (p ProtocolProviderRecords) Put(ctx context.Context, to PeerID, record PeerRecord) error {
	return nil
}

type ProtocolIPNS struct {
	Host host.Host
}

var _ kad.Protocol[key.Key256, PeerID, ma.Multiaddr, PeerRecord] = ProtocolIPNS{}

func (p ProtocolIPNS) Get(ctx context.Context, to PeerID, target key.Key256) (kad.Response[key.Key256, PeerID, ma.Multiaddr, PeerRecord], error) {
	//...
	return nil, nil
}

func (p ProtocolIPNS) Put(ctx context.Context, to PeerID, record PeerRecord) error {
	return nil
}
