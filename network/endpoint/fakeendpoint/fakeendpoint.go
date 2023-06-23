package fakeendpoint

import (
	"context"
	"fmt"
	"time"

	ba "github.com/libp2p/go-libp2p-kad-dht/events/action/basicaction"
	"github.com/libp2p/go-libp2p-kad-dht/events/planner"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler"
	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint"
	"github.com/libp2p/go-libp2p-kad-dht/network/message"
	"github.com/libp2p/go-libp2p-kad-dht/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/libp2p/go-libp2p/core/network"
)

type FakeEndpoint struct {
	self  address.NodeID
	sched scheduler.Scheduler // client

	peerstore      map[string]address.NodeID
	connStatus     map[string]network.Connectedness
	serverProtos   map[address.ProtocolID]endpoint.RequestHandlerFn // server
	streamFollowup map[endpoint.StreamID]endpoint.ResponseHandlerFn // client
	streamTimeout  map[endpoint.StreamID]planner.PlannedAction      // client

	router *FakeRouter
}

var _ endpoint.NetworkedEndpoint = (*FakeEndpoint)(nil)
var _ endpoint.SimEndpoint = (*FakeEndpoint)(nil)

func NewFakeEndpoint(self address.NodeID, sched scheduler.Scheduler, router *FakeRouter) *FakeEndpoint {
	e := &FakeEndpoint{
		self:         self,
		sched:        sched,
		serverProtos: make(map[address.ProtocolID]endpoint.RequestHandlerFn),

		peerstore:  make(map[string]address.NodeID),
		connStatus: make(map[string]network.Connectedness),

		streamFollowup: make(map[endpoint.StreamID]endpoint.ResponseHandlerFn),
		streamTimeout:  make(map[endpoint.StreamID]planner.PlannedAction),

		router: router,
	}
	if router != nil {
		router.AddPeer(self, e, sched)
	}
	return e
}

func (e *FakeEndpoint) DialPeer(ctx context.Context, id address.NodeID) error {
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

// MaybeAddToPeerstore adds the given address to the peerstore. FakeEndpoint
// doesn't take into account the ttl.
func (e *FakeEndpoint) MaybeAddToPeerstore(ctx context.Context, id address.NodeID, ttl time.Duration) error {
	strNodeID := id.String()
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

func (e *FakeEndpoint) SendRequestHandleResponse(ctx context.Context,
	protoID address.ProtocolID, id address.NodeID, req message.MinKadMessage,
	resp message.MinKadMessage, timeout time.Duration, handleResp endpoint.ResponseHandlerFn) {

	ctx, span := util.StartSpan(ctx, "SendRequestHandleResponse",
		trace.WithAttributes(attribute.Stringer("id", id)),
	)
	defer span.End()

	if handleResp == nil {
		span.RecordError(fmt.Errorf("handleResp is nil"))
		return
	}

	if err := e.DialPeer(ctx, id); err != nil {
		span.RecordError(err)
		handleResp(ctx, nil, err)
		return
	}

	// send request
	sid, err := e.router.SendMessage(ctx, e.self, id, protoID, 0, req)
	if err != nil {
		span.RecordError(err)
		handleResp(ctx, nil, err)
		return
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
				if !ok {
					span.RecordError(fmt.Errorf("no followup for stream %d", sid))
					return
				}
				handleFn(ctx, nil, endpoint.ErrTimeout)
			}))
	}
}

// Peerstore functions
func (e *FakeEndpoint) Connectedness(id address.NodeID) network.Connectedness {
	if s, ok := e.connStatus[id.String()]; !ok {
		return network.NotConnected
	} else {
		return s
	}
}

func (e *FakeEndpoint) NetworkAddress(id address.NodeID) (address.NodeID, error) {
	if ai, ok := e.peerstore[id.String()]; ok {
		return ai, nil
	}
	return nil, endpoint.ErrUnknownPeer
}

func (e *FakeEndpoint) KadKey() key.KadKey {
	return e.self.Key()
}

func (e *FakeEndpoint) HandleMessage(ctx context.Context, id address.NodeID,
	protoID address.ProtocolID, sid endpoint.StreamID, msg message.MinKadMessage) {

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

		resp, ok := msg.(message.MinKadResponseMessage)
		var err error
		if !ok {
			err = fmt.Errorf("expected response message, got %T", msg)
		}
		e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
			followup(ctx, resp, err)
		}))
		return
	}

	if _, ok := e.serverProtos[protoID]; ok {
		// it isn't a response, so treat it as a request
		resp, err := e.serverProtos[protoID](ctx, id, msg)
		if err != nil {
			span.RecordError(err)
			return
		}
		e.router.SendMessage(ctx, e.self, id, protoID, sid, resp)
	}
}

func (e *FakeEndpoint) AddRequestHandler(protoID address.ProtocolID,
	reqHandler endpoint.RequestHandlerFn) {
	e.serverProtos[protoID] = reqHandler
}

func (e *FakeEndpoint) RemoveRequestHandler(protoID address.ProtocolID) {
	delete(e.serverProtos, protoID)
}
