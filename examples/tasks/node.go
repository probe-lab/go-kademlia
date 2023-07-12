package main

import (
	"context"
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

func (f *FakeNode) Closest(ctx context.Context, kk key.KadKey, n int) ([]address.NodeAddr, error) {
	nearest, err := f.rt.NearestPeers(ctx, kk, n)
	if err != nil {
		return nil, fmt.Errorf("failed to look up closer peers: %w", err)
	}

	addrs := make([]address.NodeAddr, 0, len(nearest))
	for i := range nearest {
		addr, ok := f.peerstore[nearest[i]]
		if !ok {
			continue
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func (f *FakeNode) HandleMessage(ctx context.Context, msg message.MinKadRequestMessage) (message.MinKadResponseMessage, error) {
	switch tmsg := msg.(type) {
	case *FindNodeRequest:
		closer, err := f.Closest(ctx, tmsg.Key, 2)
		if err != nil {
			return nil, fmt.Errorf("failed to look up closer peers: %w", err)
		}
		return &FindNodeResponse{Key: tmsg.Key, CloserPeers: closer}, nil
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

func (r *MessageRouter) SendMessage(ctx context.Context, addr address.NodeAddr, msg message.MinKadRequestMessage, onSuccess func(context.Context, address.NodeAddr, message.MinKadResponseMessage), onFailure func(context.Context, address.NodeAddr, error)) {
	go func() {
		n, ok := r.nodes[addr.NodeID()]
		if !ok {
			onFailure(ctx, addr, fmt.Errorf("unroutable address"))
			return
		}

		// this would be a network request in reality so fake a tiny delay
		time.Sleep(time.Millisecond)

		resp, err := n.HandleMessage(ctx, msg)
		if err != nil {
			onFailure(ctx, addr, fmt.Errorf("node responded with error: %w", err))
			return
		}

		onSuccess(ctx, addr, resp)
	}()
}

type FindNodeRequest struct {
	Key key.KadKey
}

func (r FindNodeRequest) Target() key.KadKey {
	return r.Key
}

func (FindNodeRequest) EmptyResponse() message.MinKadResponseMessage {
	return &FindNodeResponse{}
}

type FindNodeResponse struct {
	Key         key.KadKey
	CloserPeers []address.NodeAddr
}

func (r *FindNodeResponse) CloserNodes() []address.NodeAddr {
	return r.CloserPeers
}
