package sim

import (
	"context"

	ba "github.com/plprobelab/go-kademlia/events/action/basicaction"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
)

type Router[K kad.Key[K]] struct {
	currStream endpoint.StreamID
	peers      map[string]endpoint.SimEndpoint[K]
	scheds     map[string]scheduler.Scheduler
}

func NewRouter[K kad.Key[K]]() *Router[K] {
	return &Router[K]{
		currStream: 1,
		peers:      make(map[string]endpoint.SimEndpoint[K]),
		scheds:     make(map[string]scheduler.Scheduler),
	}
}

func (r *Router[K]) AddPeer(id address.NodeID[K], peer endpoint.SimEndpoint[K], sched scheduler.Scheduler) {
	r.peers[id.String()] = peer
	r.scheds[id.String()] = sched
}

func (r *Router[K]) RemovePeer(id address.NodeID[K]) {
	delete(r.peers, id.String())
	delete(r.scheds, id.String())
}

func (r *Router[K]) SendMessage(ctx context.Context, from, to address.NodeID[K],
	protoID address.ProtocolID, sid endpoint.StreamID,
	msg message.MinKadMessage,
) (endpoint.StreamID, error) {
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
