package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routing"
)

type FakeNode struct {
	addr      address.NodeAddr
	rt        routing.Table
	peerstore map[address.NodeID]address.NodeAddr
}

func (f *FakeNode) Addr() address.NodeAddr {
	return f.addr
}

func (f *FakeNode) Key() key.KadKey {
	return f.addr.NodeID().Key()
}

func (f *FakeNode) NodeID() address.NodeID {
	return f.addr.NodeID()
}

func (f *FakeNode) AddNodeAddr(ctx context.Context, addr address.NodeAddr) {
	f.rt.AddPeer(ctx, addr.NodeID())
	f.peerstore[addr.NodeID()] = addr
}

func (f *FakeNode) AddressOf(id address.NodeID) address.NodeAddr {
	return f.peerstore[id]
}

func (f *FakeNode) Closest(ctx context.Context, kk key.KadKey, n int) ([]address.NodeID, error) {
	return f.rt.NearestPeers(ctx, kk, n)
}

func (f *FakeNode) HandleMessage(ctx context.Context, msg message.MinKadRequestMessage) (message.MinKadResponseMessage, error) {
	switch tmsg := msg.(type) {
	case *FindNodeRequest:
		closer, err := f.Closest(ctx, tmsg.NodeID.Key(), 2)
		if err != nil {
			return nil, fmt.Errorf("failed to look up closer peers: %w", err)
		}

		addrs := make([]address.NodeAddr, 0, len(closer))
		for i := range closer {
			addr, ok := f.peerstore[closer[i]]
			if !ok {
				continue
			}
			addrs = append(addrs, addr)
		}

		return &FindNodeResponse{NodeID: tmsg.NodeID, CloserPeers: addrs}, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %T", tmsg)
	}
}

type MessageRouter struct {
	nodes map[address.NodeID]*FakeNode
}

func NewMessageRouter(nodes []*FakeNode) *MessageRouter {
	mr := &MessageRouter{
		nodes: make(map[address.NodeID]*FakeNode),
	}

	for _, n := range nodes {
		mr.nodes[n.addr.NodeID()] = n
	}

	return mr
}

var ErrNoKnownAddress = errors.New("no known address")

func (r *MessageRouter) SendMessage(ctx context.Context, addr address.NodeAddr, msg message.MinKadRequestMessage) (message.MinKadResponseMessage, error) {
	n, ok := r.nodes[addr.NodeID()]
	if !ok {
		return nil, ErrNoKnownAddress
	}

	// this would be a network request in reality so fake a tiny delay
	time.Sleep(time.Millisecond)

	return n.HandleMessage(ctx, msg)
}

type FindNodeRequest struct {
	NodeID address.NodeID
}

func (r FindNodeRequest) Target() key.KadKey {
	return r.NodeID.Key()
}

func (FindNodeRequest) EmptyResponse() message.MinKadResponseMessage {
	return &FindNodeResponse{}
}

type FindNodeResponse struct {
	NodeID      address.NodeID // node we were looking for
	CloserPeers []address.NodeAddr
}

func (r *FindNodeResponse) CloserNodes() []address.NodeAddr {
	return r.CloserPeers
}
