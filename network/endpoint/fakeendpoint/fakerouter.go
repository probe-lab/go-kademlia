package fakeendpoint

import (
	"context"

	ba "github.com/libp2p/go-libp2p-kad-dht/events/action/basicaction"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint"
	"github.com/libp2p/go-libp2p-kad-dht/network/message"
)

type FakeRouter struct {
	currStream endpoint.StreamID
	peers      map[string]endpoint.SimEndpoint
	scheds     map[string]scheduler.Scheduler
}

func NewFakeRouter() *FakeRouter {
	return &FakeRouter{
		currStream: 1,
		peers:      make(map[string]endpoint.SimEndpoint),
		scheds:     make(map[string]scheduler.Scheduler),
	}
}

func (r *FakeRouter) AddPeer(id address.NodeID, peer endpoint.SimEndpoint, sched scheduler.Scheduler) {
	r.peers[id.String()] = peer
	r.scheds[id.String()] = sched
}

func (r *FakeRouter) RemovePeer(id address.NodeID) {
	delete(r.peers, id.String())
	delete(r.scheds, id.String())
}

func (r *FakeRouter) SendMessage(ctx context.Context, from, to address.NodeID,
	protoID address.ProtocolID, sid endpoint.StreamID,
	msg message.MinKadMessage) (endpoint.StreamID, error) {
	if _, ok := r.peers[to.String()]; !ok {
		return 0, endpoint.ErrUnknownPeer
	}
	if sid == 0 {
		sid = r.currStream
		r.currStream++
	}
	r.scheds[to.String()].EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
		r.peers[to.String()].HandleMessage(ctx, from, protoID, sid, msg)
	}))
	return sid, nil
}
