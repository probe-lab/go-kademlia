package sim

import (
	"context"

	"github.com/probe-lab/go-kademlia/event"
	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/network/address"
	"github.com/probe-lab/go-kademlia/network/endpoint"
)

type Router[K kad.Key[K], A kad.Address[A]] struct {
	currStream endpoint.StreamID
	peers      map[string]SimEndpoint[K, A]
	scheds     map[string]event.Scheduler
}

func NewRouter[K kad.Key[K], A kad.Address[A]]() *Router[K, A] {
	return &Router[K, A]{
		currStream: 1,
		peers:      make(map[string]SimEndpoint[K, A]),
		scheds:     make(map[string]event.Scheduler),
	}
}

func (r *Router[K, A]) AddPeer(id kad.NodeID[K], peer SimEndpoint[K, A], sched event.Scheduler) {
	r.peers[id.String()] = peer
	r.scheds[id.String()] = sched
}

func (r *Router[K, A]) RemovePeer(id kad.NodeID[K]) {
	delete(r.peers, id.String())
	delete(r.scheds, id.String())
}

func (r *Router[K, A]) SendMessage(ctx context.Context, from, to kad.NodeID[K],
	protoID address.ProtocolID, sid endpoint.StreamID,
	msg kad.Message,
) (endpoint.StreamID, error) {
	if _, ok := r.peers[to.String()]; !ok {
		return 0, endpoint.ErrUnknownPeer
	}
	if sid == 0 {
		sid = r.currStream
		r.currStream++
	}
	r.scheds[to.String()].EnqueueAction(ctx, event.BasicAction(func(ctx context.Context) {
		r.peers[to.String()].HandleMessage(ctx, from, protoID, sid, msg)
	}))
	return sid, nil
}
