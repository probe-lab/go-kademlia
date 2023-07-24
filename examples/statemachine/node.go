package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routing"
)

type FakeNode[K kad.Key[K]] struct {
	addr      address.NodeAddr[K]
	rt        routing.Table[K]
	peerstore map[address.NodeID[K]]address.NodeAddr[K]
}

func (f *FakeNode[K]) Addr() address.NodeAddr[K] {
	return f.addr
}

func (f *FakeNode[K]) Key() K {
	return f.addr.NodeID().Key()
}

func (f *FakeNode[K]) NodeID() address.NodeID[K] {
	return f.addr.NodeID()
}

func (f *FakeNode[K]) AddNodeAddr(ctx context.Context, addr address.NodeAddr[K]) {
	f.rt.AddPeer(ctx, addr.NodeID())
	f.peerstore[addr.NodeID()] = addr
}

func (f *FakeNode[K]) AddressOf(id address.NodeID[K]) address.NodeAddr[K] {
	return f.peerstore[id]
}

func (f *FakeNode[K]) Closest(ctx context.Context, kk K, n int) ([]address.NodeID[K], error) {
	return f.rt.NearestPeers(ctx, kk, n)
}

func (f *FakeNode[K]) HandleMessage(ctx context.Context, msg message.MinKadRequestMessage[K]) (message.MinKadResponseMessage[K], error) {
	switch tmsg := msg.(type) {
	case *FindNodeRequest[K]:
		closer, err := f.Closest(ctx, tmsg.NodeID.Key(), 2)
		if err != nil {
			return nil, fmt.Errorf("failed to look up closer peers: %w", err)
		}

		addrs := make([]address.NodeAddr[K], 0, len(closer))
		for i := range closer {
			addr, ok := f.peerstore[closer[i]]
			if !ok {
				continue
			}
			addrs = append(addrs, addr)
		}

		return &FindNodeResponse[K]{NodeID: tmsg.NodeID, CloserPeers: addrs}, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %T", tmsg)
	}
}

type MessageRouter[K kad.Key[K]] struct {
	nodes map[address.NodeID[K]]*FakeNode[K]
}

func NewMessageRouter[K kad.Key[K]](nodes []*FakeNode[K]) *MessageRouter[K] {
	mr := &MessageRouter[K]{
		nodes: make(map[address.NodeID[K]]*FakeNode[K]),
	}

	for _, n := range nodes {
		mr.nodes[n.addr.NodeID()] = n
	}

	return mr
}

var ErrNoKnownAddress = errors.New("no known address")

func (r *MessageRouter[K]) SendMessage(ctx context.Context, addr address.NodeAddr[K], msg message.MinKadRequestMessage[K]) (message.MinKadResponseMessage[K], error) {
	n, ok := r.nodes[addr.NodeID()]
	if !ok {
		return nil, ErrNoKnownAddress
	}

	// this would be a network request in reality so fake a tiny delay
	time.Sleep(time.Millisecond)

	return n.HandleMessage(ctx, msg)
}

type FindNodeRequest[K kad.Key[K]] struct {
	NodeID address.NodeID[K]
}

func (r FindNodeRequest[K]) Target() K {
	return r.NodeID.Key()
}

func (FindNodeRequest[K]) EmptyResponse() message.MinKadResponseMessage[K] {
	return &FindNodeResponse[K]{}
}

type FindNodeResponse[K kad.Key[K]] struct {
	NodeID      address.NodeID[K] // node we were looking for
	CloserPeers []address.NodeAddr[K]
}

func (r *FindNodeResponse[K]) CloserNodes() []address.NodeAddr[K] {
	return r.CloserPeers
}
