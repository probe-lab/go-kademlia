package simplequery

import (
	"context"
	"errors"
	"strconv"
	"time"

	ba "github.com/plprobelab/go-kademlia/events/action/basicaction"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	message "github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routingtable"
	"github.com/plprobelab/go-kademlia/util"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// note that the returned []address.NodeID are expected to be of the same type
// as the type returned by the routing table's NearestPeers method. the
// address.NodeID returned by resp.CloserNodes() is not necessarily of the same
// type as the one returned by the routing table's NearestPeers method. so
// address.NodeID s may need to be converted in this function.
type HandleResultFn func(context.Context, address.NodeID,
	message.MinKadResponseMessage) (bool, []address.NodeID)

type NotifyFailureFn func(context.Context)

type SimpleQuery struct {
	ctx          context.Context
	done         bool
	protoID      address.ProtocolID
	req          message.MinKadRequestMessage
	concurrency  int
	peerstoreTTL time.Duration
	timeout      time.Duration

	msgEndpoint endpoint.Endpoint
	rt          routingtable.RoutingTable
	sched       scheduler.Scheduler

	inflightRequests int // requests that are either in flight or scheduled
	peerlist         *peerList

	// response handling
	handleResultFn HandleResultFn
	// failure callback
	notifyFailureFn NotifyFailureFn
}

// NewSimpleQuery creates a new SimpleQuery. It initializes the query by adding
// the closest peers to the target key from the provided routing table to the
// query's peerlist. It sends `concurreny` requests events to the provided event
// queue. The requests events and followup events are handled by the event queue
// reader, and the parameters to these events are determined by the query's
// parameters. The query keeps track of the closest known peers to the target
// key, and the peers that have been queried so far.
func NewSimpleQuery(ctx context.Context, req message.MinKadRequestMessage,
	opts ...Option) (*SimpleQuery, error) {
	ctx, span := util.StartSpan(ctx, "SimpleQuery.NewSimpleQuery",
		trace.WithAttributes(attribute.String("Target", req.Target().Hex())))
	defer span.End()

	// apply options
	var cfg Config
	if err := cfg.Apply(append([]Option{DefaultConfig}, opts...)...); err != nil {
		span.RecordError(err)
		return nil, err
	}

	// get the closest peers to the target from the routing table
	closestPeers, err := cfg.RoutingTable.NearestPeers(ctx, req.Target(),
		cfg.NumberUsefulCloserPeers)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	if len(closestPeers) == 0 {
		err := errors.New("no peers in routing table")
		span.RecordError(err)
		return nil, err
	}

	// create new empty peerlist
	pl := newPeerList(req.Target(), cfg.Endpoint)
	// add the closest peers to peerlist
	pl.addToPeerlist(closestPeers)

	q := &SimpleQuery{
		ctx:             ctx,
		req:             req,
		protoID:         cfg.ProtocolID,
		concurrency:     cfg.Concurrency,
		timeout:         cfg.RequestTimeout,
		peerstoreTTL:    cfg.PeerstoreTTL,
		rt:              cfg.RoutingTable,
		msgEndpoint:     cfg.Endpoint,
		sched:           cfg.Scheduler,
		handleResultFn:  cfg.HandleResultsFunc,
		notifyFailureFn: cfg.NotifyFailureFunc,
		peerlist:        pl,
	}

	// add concurrency number of requests to eventqueue
	q.enqueueNewRequests(ctx)

	return q, nil
}

// checkIfDone cheks if the query is done, and return an error if it is done.
func (q *SimpleQuery) checkIfDone() error {
	if q.done {
		// query is done, don't send any more requests
		return errors.New("query done")
	}
	if q.ctx.Err() != nil {
		q.done = true
		return q.ctx.Err()
	}
	return nil
}

// enqueueNewRequests adds the maximal number of requests to the scheduler's
// event queue. The maximal number of conccurent requests is limited by the
// concurrency factor and by the number of queued peers in the peerlist.
func (q *SimpleQuery) enqueueNewRequests(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "SimpleQuery.enqueueNewRequests")
	defer span.End()

	// we always want to have the maximal number of requests in flight
	newRequestsToSend := q.concurrency - q.inflightRequests
	if q.peerlist.queuedCount < newRequestsToSend {
		newRequestsToSend = q.peerlist.queuedCount
	}

	if newRequestsToSend == 0 && q.inflightRequests == 0 {
		// no more requests to send and no requests in flight, query has failed
		// and is done
		q.done = true
		span.AddEvent("all peers queried")
		q.notifyFailureFn(ctx)
		return
	}

	span.AddEvent("newRequestsToSend: " + strconv.Itoa(newRequestsToSend) +
		" q.inflightRequests: " + strconv.Itoa(q.inflightRequests))

	for i := 0; i < newRequestsToSend; i++ {
		// add new pending request(s) for this query to eventqueue
		q.sched.EnqueueAction(ctx, ba.BasicAction(q.newRequest))

	}
	// increase number of inflight requests. Note that it counts both queued
	// requests and requests in flight
	q.inflightRequests += newRequestsToSend
	span.AddEvent("Enqueued " + strconv.Itoa(newRequestsToSend) +
		" SimpleQuery.newRequest")
}

// newRequest sends a request to the closest peer that hasn't been queried yet.
func (q *SimpleQuery) newRequest(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "SimpleQuery.newRequest")
	defer span.End()

	if err := q.checkIfDone(); err != nil {
		span.RecordError(err)
		// decrease counter, because there is one less request queued
		q.inflightRequests--
		return
	}

	// get the closest peer from target that hasn't been queried yet
	id := q.peerlist.popClosestQueued()
	if id == nil {
		// the peer list is empty, we don't have any more peers to query. This
		// shouldn't happen because enqueueNewRequests doesn't enqueue more
		// requests than there are queued peers in the peerlist
		q.inflightRequests--
		return
	}
	span.AddEvent("Peer selected: " + id.String())

	// this function will be queued when a response is received, an error
	// occures or the request times out (with appropriate parameters)
	handleResp := func(ctx context.Context, resp message.MinKadResponseMessage,
		err error) {
		if err != nil {
			q.requestError(ctx, id, err)
		} else {
			q.handleResponse(ctx, id, resp)
		}
	}

	// send request
	err := q.msgEndpoint.SendRequestHandleResponse(ctx, q.protoID, id, q.req,
		q.req.EmptyResponse(), q.timeout, handleResp)
	if err != nil {
		// there was an error before the request was sent, handle it
		span.RecordError(err)
		q.requestError(ctx, id, err)
	}
}

// handleResponse handles a response to a past query request
func (q *SimpleQuery) handleResponse(ctx context.Context, id address.NodeID,
	resp message.MinKadResponseMessage) {
	ctx, span := util.StartSpan(ctx, "SimpleQuery.handleResponse",
		trace.WithAttributes(attribute.String("Target", q.req.Target().Hex()),
			attribute.String("From Peer", id.String())))
	defer span.End()

	if err := q.checkIfDone(); err != nil {
		// request completed or was cancelled was the message was in flight,
		// don't handle the message
		span.RecordError(err)
		return
	}

	if resp == nil {
		err := errors.New("nil response")
		span.RecordError(err)
		q.requestError(ctx, id, err)
		return
	}

	closerPeers := resp.CloserNodes()
	if len(closerPeers) > 0 {
		// consider that remote peer is behaving correctly if it returns
		// at least 1 peer. We add it to our routing table only if it behaves
		// as expected (we don't want to add unresponsive nodes to the rt)
		q.rt.AddPeer(ctx, id)
	}

	q.inflightRequests--

	// set peer as queried in the peerlist
	q.peerlist.queriedPeer(id)

	// handle the response using the function provided by the caller, this
	// function decides whether the query should terminate and returns the list
	// of useful nodes that should be queried next
	stop, usefulNodeIDs := q.handleResultFn(ctx, id, resp)
	if stop {
		// query is done, don't send any more requests
		span.AddEvent("query over")
		q.done = true
		return
	}

	// remove all occurneces of q.self from usefulNodeIDs
	writeIndex := 0
	for _, id := range usefulNodeIDs {
		if c, err := q.rt.Self().Compare(id.Key()); err == nil && c != 0 {
			// id is valid and isn't self
			usefulNodeIDs[writeIndex] = id
			writeIndex++
		} else if err != nil {
			span.AddEvent("wrong KadKey lenght")
		} else {
			span.AddEvent("never add self to query peerlist")
		}
	}
	usefulNodeIDs = usefulNodeIDs[:writeIndex]

	// add usefulNodeIDs to peerlist
	q.peerlist.addToPeerlist(usefulNodeIDs)

	// enqueue new query requests to the event loop (usually 1)
	q.enqueueNewRequests(ctx)
}

// requestError handle an error that occured while sending a request or
// receiving a response.
func (q *SimpleQuery) requestError(ctx context.Context, id address.NodeID, err error) {
	ctx, span := util.StartSpan(ctx, "SimpleQuery.requestError",
		trace.WithAttributes(attribute.String("PeerID", id.String()),
			attribute.String("Error", err.Error())))
	defer span.End()

	// the request isn't in flight anymore since it failed
	q.inflightRequests--

	if q.ctx.Err() == nil {
		// remove peer from routing table unless context was cancelled. We don't
		// want to keep peers that timed out or peers that returned nil/invalid
		// responses.
		q.rt.RemoveKey(ctx, id.Key())
	}

	if err := q.checkIfDone(); err != nil {
		span.RecordError(err)
		return
	}

	// set peer as unreachable in the peerlist
	q.peerlist.unreachablePeer(id)

	// enqueue new query requests to the event loop (usually 1)
	q.enqueueNewRequests(ctx)
}
