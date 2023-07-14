package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/message"
)

type KademliaHandler struct {
	self           *FakeNode
	qp             *QueryPool
	mr             *MessageRouter
	notify         chan struct{} // channel to notify there is potentially work to do
	outboundEvents chan Event
	inboundEvents  chan Event
}

func NewKademliaHandler(node *FakeNode, mr *MessageRouter) *KademliaHandler {
	qp := NewQueryPool(node, mr)
	return &KademliaHandler{
		self:           node,
		qp:             qp,
		mr:             mr,
		notify:         make(chan struct{}, 20),
		outboundEvents: make(chan Event, 20),
		inboundEvents:  make(chan Event, 20),
	}
}

func (k *KademliaHandler) Start(ctx context.Context) <-chan Event {
	go k.mainloop(ctx)
	return k.outboundEvents
}

func (k *KademliaHandler) mainloop(ctx context.Context) {
	for {
		trace("KademliaHandler.mainloop: waiting for inbound events")

		// wait for inbound events to trigger state changes
		select {
		case <-ctx.Done():
			return
		case ev := <-k.inboundEvents:
			switch tev := ev.(type) {
			case *UnroutablePeerEvent:

			case *MessageFailedEvent:

			case *MessageResponseEvent:
				k.onMessageSuccess(ctx, tev.QueryID, tev.NodeID, tev.Response)
			default:
				panic(fmt.Sprintf("unexpected event: %T", tev))
			}
		case <-k.notify:
			// got a hint that there is work to do
		}

		trace("KademliaHandler.mainloop: advancing query state machine")
		// attempt to advance the query state machine
		state := k.qp.Advance(ctx)
		traceReturnState("main", state)
		switch st := state.(type) {
		case *QueryPoolWaiting:
			trace("Query pool is waiting for %d", st.QueryID)
		case *QueryPoolWaitingMessage:
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

func (h *KademliaHandler) attemptSendMessage(ctx context.Context, to address.NodeID, msg message.MinKadRequestMessage, queryID QueryID) {
	trace("KademliaHandler.attemptSendMessage")
	go func() {
		// HACK: assume always works
		addr := h.self.AddressOf(to)
		resp, err := h.mr.SendMessage(ctx, addr, msg)
		if err != nil {
			if errors.Is(err, ErrNoKnownAddress) {
				// here we can notify that the peer is unroutable, which would feed into peerstore and routing table
				h.inboundEvents <- &UnroutablePeerEvent{NodeID: to}
			}
			h.inboundEvents <- &MessageFailedEvent{NodeID: to, QueryID: queryID}
			return
		}

		h.inboundEvents <- &MessageResponseEvent{NodeID: to, QueryID: queryID, Response: resp}
	}()
}

func (k *KademliaHandler) onMessageSuccess(ctx context.Context, queryID QueryID, node address.NodeID, resp message.MinKadResponseMessage) {
	// HACK: add closer nodes to peer store
	// TODO: make this an inbound event
	for _, cn := range resp.CloserNodes() {
		k.self.AddNodeAddr(ctx, cn)
	}

	// notify caller so they have chance to stop query
	k.outboundEvents <- &KademliaOutboundQueryProgressedEvent{
		NodeID:   node,
		QueryID:  queryID,
		Response: resp,
	}
	k.qp.onMessageSuccess(ctx, queryID, node, resp)
}

func (k *KademliaHandler) StartQuery(ctx context.Context, msg message.MinKadRequestMessage) (QueryID, error) {
	trace("KademliaHandler.StartQuery")
	// If not in peer store then query the Kademlia dht
	queryID, err := k.qp.AddQuery(ctx, msg.Target(), msg)
	if err != nil {
		return InvalidQueryID, fmt.Errorf("failed to start query: %w", err)
	}

	k.notify <- struct{}{}
	return queryID, nil
}

func (k *KademliaHandler) StopQuery(ctx context.Context, queryID QueryID) error {
	trace("KademliaHandler.StopQuery")
	return k.qp.StopQuery(ctx, queryID)
}

// Events emitted by the Kademlia Handler

type KademliaRoutingUpdatedEvent struct{}

type KademliaOutboundQueryProgressedEvent struct {
	NodeID   address.NodeID
	QueryID  QueryID
	Response message.MinKadResponseMessage
}

type KademliaUnroutablePeerEvent struct{}

type KademliaRoutablePeerEvent struct{}

// Internal events for the Kademlia Handler

type UnroutablePeerEvent struct {
	NodeID address.NodeID
}

type MessageFailedEvent struct {
	NodeID  address.NodeID
	QueryID QueryID
}

type MessageResponseEvent struct {
	NodeID   address.NodeID
	QueryID  QueryID
	Response message.MinKadResponseMessage
}
