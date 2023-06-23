package simplequery

import (
	"context"
	"errors"
	"strconv"
	"time"

	ba "github.com/libp2p/go-libp2p-kad-dht/events/action/basicaction"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler"
	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint"
	message "github.com/libp2p/go-libp2p-kad-dht/network/message"
	"github.com/libp2p/go-libp2p-kad-dht/routingtable"
	"github.com/libp2p/go-libp2p-kad-dht/util"

	"github.com/libp2p/go-libp2p/core/peerstore"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	// MAGIC: takes the default value from the DHT constants
	NClosestPeers = 20

	// MAGIC: default peerstore TTL for newly discovered peers
	QueuedPeersPeerstoreTTL = peerstore.TempAddrTTL
)

// TODO: Options: NClosestPeers, QueuedPeersPeerstoreTTL, concurrency int, timeout time.Duration

type HandleResultFn func(context.Context, address.NodeID, message.MinKadResponseMessage) (bool, []address.NodeID)

type SimpleQuery struct {
	ctx         context.Context
	done        bool
	kadid       key.KadKey
	protoID     address.ProtocolID
	req         message.MinKadMessage
	resp        message.MinKadResponseMessage
	concurrency int
	timeout     time.Duration

	msgEndpoint endpoint.Endpoint
	rt          routingtable.RoutingTable
	sched       scheduler.Scheduler

	inflightRequests int // requests that are either in flight or scheduled
	peerlist         *peerList

	// success condition
	handleResultFn HandleResultFn
}

// NewSimpleQuery creates a new SimpleQuery. It initializes the query by adding
// the closest peers to the target key from the provided routing table to the
// query's peerlist. It sends `concurreny` requests events to the provided event
// queue. The requests events and followup events are handled by the event queue
// reader, and the parameters to these events are determined by the query's
// parameters. The query keeps track of the closest known peers to the target
// key, and the peers that have been queried so far.
func NewSimpleQuery(ctx context.Context, kadid key.KadKey, proto address.ProtocolID,
	req message.MinKadMessage, resp message.MinKadResponseMessage, concurrency int,
	timeout time.Duration, msgEndpoint endpoint.Endpoint, rt routingtable.RoutingTable,
	sched scheduler.Scheduler, handleResultFn HandleResultFn) *SimpleQuery {

	ctx, span := util.StartSpan(ctx, "SimpleQuery.NewSimpleQuery",
		trace.WithAttributes(attribute.String("Target", kadid.Hex())))
	defer span.End()

	closestPeers, err := rt.NearestPeers(ctx, kadid, NClosestPeers)
	if err != nil {
		span.RecordError(err)
		return nil
	}

	pl := newPeerList(kadid)
	pl.addToPeerlist(closestPeers)

	q := &SimpleQuery{
		ctx:              ctx,
		req:              req,
		resp:             resp,
		kadid:            kadid,
		protoID:          proto,
		concurrency:      concurrency,
		timeout:          timeout,
		msgEndpoint:      msgEndpoint,
		rt:               rt,
		inflightRequests: 0,
		peerlist:         pl,
		sched:            sched,
		handleResultFn:   handleResultFn,
	}

	// we don't want more pending requests than the number of peers we can query
	requestsEvents := concurrency
	if len(closestPeers) < concurrency {
		requestsEvents = len(closestPeers)
	}
	for i := 0; i < requestsEvents; i++ {
		// add concurrency requests to the event queue
		q.sched.EnqueueAction(ctx, ba.BasicAction(q.newRequest))
	}
	span.AddEvent("Enqueued " + strconv.Itoa(requestsEvents) + " SimpleQuery.newRequest")
	q.inflightRequests = requestsEvents

	return q
}

func (q *SimpleQuery) checkIfDone() error {
	if q.done {
		// query is done, don't send any more requests
		return errors.New("query done")
	}

	select {
	case <-q.ctx.Done():
		// query is cancelled, mark it as done
		q.done = true
		return errors.New("query cancelled")
	default:
	}
	return nil
}

func (q *SimpleQuery) newRequest(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	ctx, span := util.StartSpan(ctx, "SimpleQuery.newRequest")
	defer span.End()

	if err := q.checkIfDone(); err != nil {
		span.RecordError(err)
		q.inflightRequests--
		return
	}

	id := q.peerlist.popClosestQueued()
	if id == nil || id.String() == "" {
		// TODO: handle this case
		span.AddEvent("all peers queried")
		q.inflightRequests--
		return
	}
	span.AddEvent("peer selected: " + id.String())

	// function to be executed when a response is received
	handleResp := func(ctx context.Context, resp message.MinKadResponseMessage, err error) {
		ctx, span := util.StartSpan(ctx, "SimpleQuery.handleResp")
		defer span.End()

		if err != nil {
			span.AddEvent("got error")
			q.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
				q.requestError(ctx, id, err)
			}))
		} else {
			span.AddEvent("got response")
			q.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
				q.handleResponse(ctx, id, resp)
			}))
			span.AddEvent("Enqueued SimpleQuery.handleResponse")
		}
	}

	// send request
	q.msgEndpoint.SendRequestHandleResponse(ctx, q.protoID, id, q.req, q.resp, q.timeout, handleResp)
}

func (q *SimpleQuery) handleResponse(ctx context.Context, id address.NodeID, resp message.MinKadResponseMessage) {
	ctx, span := util.StartSpan(ctx, "SimpleQuery.handleResponse",
		trace.WithAttributes(attribute.String("Target", q.kadid.Hex()), attribute.String("From Peer", id.String())))
	defer span.End()

	if err := q.checkIfDone(); err != nil {
		span.RecordError(err)
		return
	}

	if resp == nil {
		span.AddEvent("response is nil")
		q.requestError(ctx, id, errors.New("nil response"))
		return
	}

	q.inflightRequests--

	closerPeers := resp.CloserNodes()
	if len(closerPeers) > 0 {
		// consider that remote peer is behaving correctly if it returns
		// at least 1 peer
		q.rt.AddPeer(ctx, id)
	}

	q.peerlist.updatePeerStatusInPeerlist(id, queried)

	for i, id := range closerPeers {
		c, err := id.Key().Compare(q.rt.Self())
		if err == nil && c == 0 {
			// don't add self to queries or routing table
			span.AddEvent("remote peer provided self as closer peer")
			closerPeers = append(closerPeers[:i], closerPeers[i+1:]...)
			continue
		}

		q.msgEndpoint.MaybeAddToPeerstore(ctx, id, QueuedPeersPeerstoreTTL)
	}

	stop, usefulNodeID := q.handleResultFn(ctx, id, resp)
	if stop {
		// query is done, don't send any more requests
		span.AddEvent("query over")
		q.done = true
		return
	}

	q.peerlist.addToPeerlist(usefulNodeID)

	// we always want to have the maximal number of requests in flight
	newRequestsToSend := q.concurrency - q.inflightRequests
	if q.peerlist.queuedCount < newRequestsToSend {
		newRequestsToSend = q.peerlist.queuedCount
	}

	span.AddEvent("newRequestsToSend: " + strconv.Itoa(newRequestsToSend) + " q.inflightRequests: " + strconv.Itoa(q.inflightRequests))

	for i := 0; i < newRequestsToSend; i++ {
		// add new pending request(s) for this query to eventqueue
		q.sched.EnqueueAction(ctx, ba.BasicAction(q.newRequest))

	}
	q.inflightRequests += newRequestsToSend
	span.AddEvent("Enqueued " + strconv.Itoa(newRequestsToSend) +
		" SimpleQuery.newRequest")

}

func (q *SimpleQuery) requestError(ctx context.Context, id address.NodeID, err error) {
	ctx, span := util.StartSpan(ctx, "SimpleQuery.requestError",
		trace.WithAttributes(attribute.String("PeerID", id.String()),
			attribute.String("Error", err.Error())))
	defer span.End()

	q.inflightRequests--

	if q.ctx.Err() == nil {
		// remove peer from routing table unless context was cancelled
		q.rt.RemoveKey(ctx, id.Key())
	}

	if err := q.checkIfDone(); err != nil {
		span.RecordError(err)
		return
	}

	q.peerlist.updatePeerStatusInPeerlist(id, unreachable)

	// add pending request for this query to eventqueue
	q.sched.EnqueueAction(ctx, ba.BasicAction(q.newRequest))
}
