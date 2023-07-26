package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/plprobelab/go-kademlia/kad"
)

type FakeNode[K kad.Key[K], A kad.Address[A]] struct {
	addr      kad.NodeInfo[K, A]
	rt        kad.RoutingTable[K]
	peerstore map[kad.NodeID[K]]kad.NodeInfo[K, A]
}

func (f *FakeNode[K, A]) Addr() kad.NodeInfo[K, A] {
	return f.addr
}

func (f *FakeNode[K, A]) Key() K {
	return f.addr.ID().Key()
}

func (f *FakeNode[K, A]) NodeID() kad.NodeID[K] {
	return f.addr.ID()
}

func (f *FakeNode[K, A]) AddNodeAddr(addr kad.NodeInfo[K, A]) {
	f.rt.AddNode(addr.ID())
	f.peerstore[addr.ID()] = addr
}

func (f *FakeNode[K, A]) AddressOf(id kad.NodeID[K]) kad.NodeInfo[K, A] {
	return f.peerstore[id]
}

func (f *FakeNode[K, A]) Closest(kk K, n int) []kad.NodeID[K] {
	return f.rt.NearestNodes(kk, n)
}

func (f *FakeNode[K, A]) HandleMessage(ctx context.Context, msg kad.MinKadRequestMessage[K, A]) (kad.MinKadResponseMessage[K, A], error) {
	switch tmsg := msg.(type) {
	case *FindNodeRequest[K, A]:
		closer := f.Closest(tmsg.NodeID.Key(), 2)
		addrs := make([]kad.NodeInfo[K, A], 0, len(closer))
		for i := range closer {
			addr, ok := f.peerstore[closer[i]]
			if !ok {
				continue
			}
			addrs = append(addrs, addr)
		}

		return &FindNodeResponse[K, A]{NodeID: tmsg.NodeID, CloserPeers: addrs}, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %T", tmsg)
	}
}

type MessageRouter[K kad.Key[K], A kad.Address[A]] struct {
	nodes map[kad.NodeID[K]]*FakeNode[K, A]
}

func NewMessageRouter[K kad.Key[K], A kad.Address[A]](nodes []*FakeNode[K, A]) *MessageRouter[K, A] {
	mr := &MessageRouter[K, A]{
		nodes: make(map[kad.NodeID[K]]*FakeNode[K, A]),
	}

	for _, n := range nodes {
		mr.nodes[n.addr.ID()] = n
	}

	return mr
}

var ErrNoKnownAddress = errors.New("no known address")

func (r *MessageRouter[K, A]) SendMessage(ctx context.Context, addr kad.NodeInfo[K, A], msg kad.MinKadRequestMessage[K, A]) (kad.MinKadResponseMessage[K, A], error) {
	n, ok := r.nodes[addr.ID()]
	if !ok {
		return nil, ErrNoKnownAddress
	}

	// this would be a network request in reality so fake a tiny delay
	time.Sleep(time.Millisecond)

	return n.HandleMessage(ctx, msg)
}

type FindNodeRequest[K kad.Key[K], A kad.Address[A]] struct {
	NodeID kad.NodeID[K]
}

func (r FindNodeRequest[K, A]) Target() K {
	return r.NodeID.Key()
}

func (FindNodeRequest[K, A]) EmptyResponse() kad.MinKadResponseMessage[K, A] {
	return &FindNodeResponse[K, A]{}
}

type FindNodeResponse[K kad.Key[K], A kad.Address[A]] struct {
	NodeID      kad.NodeID[K] // node we were looking for
	CloserPeers []kad.NodeInfo[K, A]
}

func (r *FindNodeResponse[K, A]) CloserNodes() []kad.NodeInfo[K, A] {
	return r.CloserPeers
}
