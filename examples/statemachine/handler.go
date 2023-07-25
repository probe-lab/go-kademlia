package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/message"
)

type KademliaHandler[K kad.Key[K], A any] struct {
	self           *FakeNode[K, A]
	qp             *QueryPool[K, A]
	mr             *MessageRouter[K, A]
	notify         chan struct{} // channel to notify there is potentially work to do
	outboundEvents chan KademliaHandlerEvent
	inboundEvents  chan KademliaHandlerInternalEvent
	startOnce      sync.Once
}

func NewKademliaHandler[K kad.Key[K], A any](node *FakeNode[K, A], mr *MessageRouter[K, A]) *KademliaHandler[K, A] {
	qp := NewQueryPool(node, mr)
	return &KademliaHandler[K, A]{
		self:           node,
		qp:             qp,
		mr:             mr,
		notify:         make(chan struct{}, 20),
		outboundEvents: make(chan KademliaHandlerEvent, 20),
		inboundEvents:  make(chan KademliaHandlerInternalEvent, 20),
	}
}

func (k *KademliaHandler[K, A]) Start(ctx context.Context) <-chan KademliaHandlerEvent {
	// ensure there is only ever one mainloop
	k.startOnce.Do(func() {
		go k.mainloop(ctx)
	})
	return k.outboundEvents
}

func (k *KademliaHandler[K, A]) mainloop(ctx context.Context) {
	for {
		trace("KademliaHandler.mainloop: waiting for inbound events")

		// wait for inbound events to trigger state changes
		select {
		case <-ctx.Done():
			return
		case ev := <-k.inboundEvents:
			switch tev := ev.(type) {
			case *UnroutablePeerEvent[K]:

			case *MessageFailedEvent[K]:

			case *MessageResponseEvent[K, A]:
				k.onMessageSuccess(ctx, tev.QueryID, tev.NodeID, tev.Response)
			default:
				panic(fmt.Sprintf("unexpected event: %T", tev))
			}
		case <-k.notify:
			// got a hint that there is work to do
		}

		trace("KademliaHandler.mainloop: advancing query state machine")
		// attempt to advance the query state machine
		// TODO: consider passing event to the Advance method so the event gets handled inside the state machine
		// instead of calling methods that change state (for example onMessageSuccess calls all the way down to the
		// peer iterator, but maybe the event could be passed down through each Advance call). This would make
		// locking simpler.
		state := k.qp.Advance(ctx)
		traceReturnState("main", state)
		switch st := state.(type) {
		case *QueryPoolWaiting:
			trace("Query pool is waiting for %d", st.QueryID)
		case *QueryPoolWaitingMessage[K, A]:
			trace("Query pool is waiting to send a message to %v", st.NodeID)
			k.attemptSendMessage(ctx, st.NodeID, st.Message, st.QueryID)
		case *QueryPoolWaitingWithCapacity:
			trace("Query pool is waiting for one or more queries")
		case *QueryPoolFinished:
			trace("Query pool has finished query %d", st.QueryID)
			trace("Stats: %+v", st.Stats)
		case *QueryPoolTimeout:
			trace("Query pool has timed out query %d", st.QueryID)
		case *QueryPoolIdle:
			trace("Query pool has no further work, exiting this demo")
		default:
			panic(fmt.Sprintf("unexpected state: %T", st))
		}

	}
}

func (k *KademliaHandler[K, A]) attemptSendMessage(ctx context.Context, to kad.NodeID[K], msg message.MinKadRequestMessage[K, A], queryID QueryID) {
	trace("KademliaHandler.attemptSendMessage")
	go func() {
		// HACK: assume always works
		addr := k.self.AddressOf(to)
		resp, err := k.mr.SendMessage(ctx, addr, msg)
		if err != nil {
			if errors.Is(err, ErrNoKnownAddress) {
				// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
				k.inboundEvents <- &UnroutablePeerEvent[K]{NodeID: to}
			}
			k.inboundEvents <- &MessageFailedEvent[K]{NodeID: to, QueryID: queryID}
			return
		}

		k.inboundEvents <- &MessageResponseEvent[K, A]{NodeID: to, QueryID: queryID, Response: resp}
	}()
}

func (k *KademliaHandler[K, A]) onMessageSuccess(ctx context.Context, queryID QueryID, node kad.NodeID[K], resp message.MinKadResponseMessage[K, A]) {
	// HACK: add closer nodes to peer store
	// TODO: make this an inbound event
	for _, cn := range resp.CloserNodes() {
		k.self.AddNodeAddr(cn)
	}

	// notify caller so they have chance to stop query
	k.outboundEvents <- &KademliaOutboundQueryProgressedEvent[K, A]{
		NodeID:   node,
		QueryID:  queryID,
		Response: resp,
	}
	k.qp.onMessageSuccess(ctx, queryID, node, resp)
}

func (k *KademliaHandler[K, A]) StartQuery(ctx context.Context, msg message.MinKadRequestMessage[K, A]) (QueryID, error) {
	trace("KademliaHandler.StartQuery")
	// If not in peer store then query the Kademlia dht
	queryID, err := k.qp.AddQuery(ctx, msg.Target(), msg)
	if err != nil {
		return InvalidQueryID, fmt.Errorf("failed to start query: %w", err)
	}

	k.notify <- struct{}{}
	return queryID, nil
}

func (k *KademliaHandler[K, A]) StopQuery(ctx context.Context, queryID QueryID) error {
	trace("KademliaHandler.StopQuery")
	return k.qp.StopQuery(ctx, queryID)
}

// Events emitted by the Kademlia Handler

type KademliaHandlerEvent interface {
	Event
	kademliaHandlerEvent()
}

type KademliaRoutingUpdatedEvent[K kad.Key[K]] struct{}

type KademliaOutboundQueryProgressedEvent[K kad.Key[K], A any] struct {
	NodeID   kad.NodeID[K]
	QueryID  QueryID
	Response message.MinKadResponseMessage[K, A]
}

type KademliaUnroutablePeerEvent[K kad.Key[K]] struct{}

type KademliaRoutablePeerEvent[K kad.Key[K]] struct{}

// kademliaHandlerEvent() ensures that only KademliaHandler events can be assigned to a KademliaHandlerEvent.
func (*KademliaRoutingUpdatedEvent[K]) kademliaHandlerEvent()             {}
func (*KademliaOutboundQueryProgressedEvent[K, A]) kademliaHandlerEvent() {}
func (*KademliaUnroutablePeerEvent[K]) kademliaHandlerEvent()             {}
func (*KademliaRoutablePeerEvent[K]) kademliaHandlerEvent()               {}

// Internal events for the Kademlia Handler

type KademliaHandlerInternalEvent interface {
	Event
	kademliaHandlerInternalEvent()
}

type UnroutablePeerEvent[K kad.Key[K]] struct {
	NodeID kad.NodeID[K]
}

type MessageFailedEvent[K kad.Key[K]] struct {
	NodeID  kad.NodeID[K]
	QueryID QueryID
}

type MessageResponseEvent[K kad.Key[K], A any] struct {
	NodeID   kad.NodeID[K]
	QueryID  QueryID
	Response message.MinKadResponseMessage[K, A]
}

func (*UnroutablePeerEvent[K]) kademliaHandlerInternalEvent()     {}
func (*MessageFailedEvent[K]) kademliaHandlerInternalEvent()      {}
func (*MessageResponseEvent[K, A]) kademliaHandlerInternalEvent() {}
