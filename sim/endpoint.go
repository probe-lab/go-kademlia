package sim

import (
	"context"
	"fmt"
	"time"

	"github.com/plprobelab/go-kademlia/key"

	"github.com/libp2p/go-libp2p/core/network"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	ba "github.com/plprobelab/go-kademlia/events/action/basicaction"
	"github.com/plprobelab/go-kademlia/events/planner"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/util"
)

// Endpoint is a single threaded endpoint implementation simulating a network.
// It simulates a network and handles message exchanges between multiple peers in a simulation.
type Endpoint[K kad.Key[K], A any] struct {
	self  kad.NodeID[K]
	sched scheduler.Scheduler // client

	peerstore      map[string]kad.NodeInfo[K, A]
	connStatus     map[string]network.Connectedness
	serverProtos   map[address.ProtocolID]endpoint.RequestHandlerFn[K]    // server
	streamFollowup map[endpoint.StreamID]endpoint.ResponseHandlerFn[K, A] // client
	streamTimeout  map[endpoint.StreamID]planner.PlannedAction            // client

	router *Router[K, A]
}

var _ endpoint.SimEndpoint[key.Key256, any] = *Endpoint[key.Key256, any](nil)

func NewEndpoint[K kad.Key[K], A any](self kad.NodeID[K], sched scheduler.Scheduler, router *Router[K, A]) *Endpoint[K, A] {
	e := &Endpoint[K, A]{
		self:         self,
		sched:        sched,
		serverProtos: make(map[address.ProtocolID]endpoint.RequestHandlerFn[K]),

		peerstore:  make(map[string]kad.NodeInfo[K, A]),
		connStatus: make(map[string]network.Connectedness),

		streamFollowup: make(map[endpoint.StreamID]endpoint.ResponseHandlerFn[K, A]),
		streamTimeout:  make(map[endpoint.StreamID]planner.PlannedAction),

		router: router,
	}
	if router != nil {
		router.AddPeer(self, e, sched)
	}
	return e
}

func (e *Endpoint[K, A]) DialPeer(ctx context.Context, id kad.NodeID[K]) error {
	_, span := util.StartSpan(ctx, "DialPeer",
		trace.WithAttributes(attribute.String("id", id.String())),
	)
	defer span.End()

	status, ok := e.connStatus[id.String()]

	if ok {
		switch status {
		case network.Connected:
			return nil
		case network.CanConnect:
			e.connStatus[id.String()] = network.Connected
			return nil
		}
	}
	span.RecordError(endpoint.ErrUnknownPeer)
	return endpoint.ErrUnknownPeer
}

// MaybeAddToPeerstore adds the given address to the peerstore. Endpoint
// doesn't take into account the ttl.
func (e *Endpoint[K, A]) MaybeAddToPeerstore(ctx context.Context, id kad.NodeInfo[K, A], ttl time.Duration) error {
	strNodeID := id.ID().String()
	_, span := util.StartSpan(ctx, "MaybeAddToPeerstore",
		trace.WithAttributes(attribute.String("self", e.self.String())),
		trace.WithAttributes(attribute.String("id", strNodeID)),
	)
	defer span.End()

	if _, ok := e.peerstore[strNodeID]; !ok {
		e.peerstore[strNodeID] = id
	}
	if _, ok := e.connStatus[strNodeID]; !ok {
		e.connStatus[strNodeID] = network.CanConnect
	}
	return nil
}

func (e *Endpoint[K, A]) SendRequestHandleResponse(ctx context.Context,
	protoID address.ProtocolID, id kad.NodeID[K], req message.MinKadMessage,
	resp message.MinKadMessage, timeout time.Duration,
	handleResp endpoint.ResponseHandlerFn[K, A],
) error {
	ctx, span := util.StartSpan(ctx, "SendRequestHandleResponse",
		trace.WithAttributes(attribute.Stringer("id", id)),
	)
	defer span.End()

	if err := e.DialPeer(ctx, id); err != nil {
		span.RecordError(err)
		e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
			handleResp(ctx, nil, err)
		}))
		return nil
	}

	// send request. id.String() is guaranteed to be in peerstore, because
	// DialPeer checks it, and an error is returned if it's not there.
	addr := e.peerstore[id.String()]

	sid, err := e.router.SendMessage(ctx, e.self, addr.ID(), protoID, 0, req)
	if err != nil {
		span.RecordError(err)
		e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
			handleResp(ctx, nil, err)
		}))
		return nil
	}
	e.streamFollowup[sid] = handleResp

	// timeout
	if timeout != 0 {
		e.streamTimeout[sid] = scheduler.ScheduleActionIn(ctx, e.sched, timeout,
			ba.BasicAction(func(ctx context.Context) {
				ctx, span := util.StartSpan(ctx, "SendRequestHandleResponse timeout",
					trace.WithAttributes(attribute.Stringer("id", id)),
				)
				defer span.End()

				handleFn, ok := e.streamFollowup[sid]
				delete(e.streamFollowup, sid)
				delete(e.streamTimeout, sid)
				if !ok || handleFn == nil {
					span.RecordError(fmt.Errorf("no followup for stream %d", sid))
					return
				}
				handleFn(ctx, nil, endpoint.ErrTimeout)
			}))
	}
	return nil
}

// Peerstore functions
func (e *Endpoint[K, A]) Connectedness(id kad.NodeID[K]) (network.Connectedness, error) {
	if s, ok := e.connStatus[id.String()]; !ok {
		return network.NotConnected, nil
	} else {
		return s, nil
	}
}

func (e *Endpoint[K, A]) NetworkAddress(id kad.NodeID[K]) (kad.NodeInfo[K, A], error) {
	if ai, ok := e.peerstore[id.String()]; ok {
		return ai, nil
	}
	if na, ok := id.(kad.NodeInfo[K, A]); ok {
		return na, nil
	}
	return nil, endpoint.ErrUnknownPeer
}

func (e *Endpoint[K, A]) KadKey() K {
	return e.self.Key()
}

func (e *Endpoint[K, A]) HandleMessage(ctx context.Context, id kad.NodeID[K],
	protoID address.ProtocolID, sid endpoint.StreamID, msg message.MinKadMessage,
) {
	_, span := util.StartSpan(ctx, "HandleMessage",
		trace.WithAttributes(attribute.Stringer("id", id),
			attribute.Int64("StreamID", int64(sid))))
	defer span.End()

	if followup, ok := e.streamFollowup[sid]; ok {
		span.AddEvent("Response to previous request")

		timeout, ok := e.streamTimeout[sid]
		if ok {
			e.sched.RemovePlannedAction(ctx, timeout)
		}
		// remove stream id from endpoint
		delete(e.streamFollowup, sid)
		delete(e.streamTimeout, sid)

		resp, ok := msg.(message.MinKadResponseMessage[K, A])
		var err error
		if ok {
			for _, p := range resp.CloserNodes() {
				e.peerstore[p.ID().String()] = p
				e.connStatus[p.ID().String()] = network.CanConnect
			}
		} else {
			err = ErrInvalidResponseType
		}
		if followup != nil {
			e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
				followup(ctx, resp, err)
			}))
		}
		return
	}

	if handler, ok := e.serverProtos[protoID]; ok && handler != nil {
		// it isn't a response, so treat it as a request
		resp, err := handler(ctx, id, msg)
		if err != nil {
			span.RecordError(err)
			return
		}
		e.router.SendMessage(ctx, e.self, id, protoID, sid, resp)
	}
}

func (e *Endpoint[K, A]) AddRequestHandler(protoID address.ProtocolID,
	req message.MinKadMessage, reqHandler endpoint.RequestHandlerFn[K],
) error {
	if reqHandler == nil {
		return endpoint.ErrNilRequestHandler
	}
	e.serverProtos[protoID] = reqHandler
	return nil
}

func (e *Endpoint[K, A]) RemoveRequestHandler(protoID address.ProtocolID) {
	delete(e.serverProtos, protoID)
}
