package simplequery

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	ss "github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/events/simulator"
	"github.com/plprobelab/go-kademlia/events/simulator/litesimulator"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	si "github.com/plprobelab/go-kademlia/network/address/stringid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	fe "github.com/plprobelab/go-kademlia/network/endpoint/fakeendpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	sm "github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/plprobelab/go-kademlia/routingtable"
	"github.com/plprobelab/go-kademlia/routingtable/simplert"
	"github.com/plprobelab/go-kademlia/server"
	"github.com/plprobelab/go-kademlia/server/basicserver"
	"github.com/stretchr/testify/require"
)

// TestTrivialQuery tests a simple query from node0 to node1. The query stops
// after the first response, because node1 doesn't return any closer peers, and
// node0 has queried all known peers.
func TestTrivialQuery(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	peerstoreTTL := 10 * time.Minute

	router := fe.NewFakeRouter()
	node0 := si.StringID("node0")
	node1 := si.StringID("node1")
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint(node0, sched0, router)
	fendpoint1 := fe.NewFakeEndpoint(node1, sched1, router)
	rt0 := simplert.NewSimpleRT(node0.Key(), 1)
	rt1 := simplert.NewSimpleRT(node1.Key(), 1)

	// make node1 a server
	server1 := basicserver.NewBasicServer(rt1, fendpoint1)
	fendpoint1.AddRequestHandler(protoID, &sm.SimMessage{}, server1.HandleRequest)

	// connect add node1 address to node0
	err := fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)
	require.NoError(t, err)
	success, err := rt0.AddPeer(ctx, node1)
	require.NoError(t, err)
	require.True(t, success)

	req := sm.NewSimRequest(node1.Key())

	queryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(1),
		WithNumberUsefulCloserPeers(2),
		WithRequestTimeout(time.Second),
		WithPeerstoreTTL(peerstoreTTL),
		WithRoutingTable(rt0),
		WithEndpoint(fendpoint0),
		WithScheduler(sched0),
	}
	_, err = NewSimpleQuery(ctx, req, queryOpts...)
	require.NoError(t, err)

	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, sched0, sched1)
	sim.Run(ctx)
}

func TestInvalidQueryOptions(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	router := fe.NewFakeRouter()
	node := si.StringID("node0")
	sched := ss.NewSimpleScheduler(clk)
	fendpoint := fe.NewFakeEndpoint(node, sched, router)
	rt := simplert.NewSimpleRT(node.Key(), 1)

	// fails because rt is not set
	invalidOpts := []Option{}
	req := sm.NewSimRequest(nil)
	_, err := NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because rt is nil
	invalidOpts = []Option{
		WithRoutingTable(nil),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because endpoint is not set
	invalidOpts = []Option{
		WithRoutingTable(rt),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because endpoint is nil
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(nil),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because scheduler is not set
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(fendpoint),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because scheduler is nil
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(fendpoint),
		WithScheduler(nil),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because HandleResultsFunc is nil
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(fendpoint),
		WithScheduler(sched),
		WithHandleResultsFunc(nil),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because WithNotifyFailureFunc is nil
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(fendpoint),
		WithScheduler(sched),
		WithNotifyFailureFunc(nil),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because NumberUsefulCloserPeers is 0
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(fendpoint),
		WithScheduler(sched),
		WithNumberUsefulCloserPeers(0),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because Concurrency is 0
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(fendpoint),
		WithScheduler(sched),
		WithConcurrency(0),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)

}

func TestElementaryQuery(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	bucketSize := 4
	peerstoreTTL := time.Minute

	router := fe.NewFakeRouter()

	nPeers := 32
	ids := make([]address.NodeAddr, nPeers)
	scheds := make([]scheduler.AwareScheduler, nPeers)
	fendpoints := make([]endpoint.SimEndpoint, nPeers)
	rts := make([]routingtable.RoutingTable, nPeers)
	servers := make([]server.Server, nPeers)
	for i := 0; i < nPeers; i++ {
		scheds[i] = ss.NewSimpleScheduler(clk)
		ids[i] = kadid.NewKadID([]byte{byte(i * 8)})
		fendpoints[i] = fe.NewFakeEndpoint(ids[i].NodeID(), scheds[i], router)
		rts[i] = simplert.NewSimpleRT(ids[i].NodeID().Key(), bucketSize)
		servers[i] = basicserver.NewBasicServer(rts[i], fendpoints[i],
			basicserver.WithNumberUsefulCloserPeers(bucketSize))
		fendpoints[i].AddRequestHandler(protoID, &sm.SimMessage{}, servers[i].HandleRequest)
	}

	// peer ids (KadIDs) are i*8 for i in [0, 32), the keyspace is 1 byte [0, 255]
	// so peers cover the whole keyspace, and they have the format XXXX X000.

	// connect peers, and add them to the routing tables
	// note: when the bucket is complete, it contains the peers with the
	// smallest identifier (KadID).
	for i := 0; i < nPeers; i++ {
		for j := i + 1; j < nPeers; j++ {
			// add peer to peerstore
			err := fendpoints[i].MaybeAddToPeerstore(ctx, ids[j], peerstoreTTL)
			require.NoError(t, err)
			// we don't require the the peer is added to the routing table,
			// because the bucket might be full already and it is fine
			_, err = rts[i].AddPeer(ctx, ids[j].NodeID())
			require.NoError(t, err)
		}
	}

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sm.NewSimRequest(ids[len(ids)-1].NodeID().Key())

	// generic query options to be used by all peers
	defaultQueryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(1),
		WithNumberUsefulCloserPeers(bucketSize),
		WithRequestTimeout(time.Second),
		WithPeerstoreTTL(peerstoreTTL),
	}

	// query options for each peer
	queryOpts := make([][]Option, nPeers)
	for i := 0; i < nPeers; i++ {
		queryOpts[i] = append(defaultQueryOpts,
			WithRoutingTable(rts[i]),
			WithEndpoint(fendpoints[i]),
			WithScheduler(scheds[i]),
		)
	}

	// handleResultsFnInfinity is called when a peer receives a response from a
	// peer, it returns false to indicate that the query should not be stopped
	// before all peers have been queried.
	handleResultsFnInfinity := func(ctx context.Context, id address.NodeID,
		resp message.MinKadResponseMessage) (bool, []address.NodeID) {
		// TODO: test that the responses are the expected ones
		ids := make([]address.NodeID, len(resp.CloserNodes()))
		for i, n := range resp.CloserNodes() {
			ids[i] = n.NodeID()
		}
		return false, ids
	}

	// the request will eventually fail because handleResultsFnInfinity always
	// return false, so the query will never stop.
	var failedQuery bool
	notifyFailureFnInfinity := func(context.Context) {
		failedQuery = true
	}

	_, err := NewSimpleQuery(ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResultsFnInfinity),
		WithNotifyFailureFunc(notifyFailureFnInfinity))...)
	require.NoError(t, err)

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, scheds...)
	// run simulation
	sim.Run(ctx)

	require.True(t, failedQuery)
}
