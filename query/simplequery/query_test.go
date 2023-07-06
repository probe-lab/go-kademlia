package simplequery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/plprobelab/go-kademlia/events/action/basicaction"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	ss "github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/events/simulator"
	"github.com/plprobelab/go-kademlia/events/simulator/litesimulator"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/addrinfo"
	"github.com/plprobelab/go-kademlia/network/address/kadaddr"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	si "github.com/plprobelab/go-kademlia/network/address/stringid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	fe "github.com/plprobelab/go-kademlia/network/endpoint/fakeendpoint"
	"github.com/plprobelab/go-kademlia/network/endpoint/libp2pendpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	sm "github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/plprobelab/go-kademlia/routing"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/server"
	"github.com/plprobelab/go-kademlia/server/basicserver"
	"github.com/stretchr/testify/require"
)

// TestTrivialQuery tests a simple query from node0 to node1. node1 responds
// with a single peer (node2) that will be in turn queried too, but node2 won't
// return any closer peers
func TestTrivialQuery(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	peerstoreTTL := 10 * time.Minute

	router := fe.NewFakeRouter()
	node0 := kadid.NewKadID([]byte{0x00})
	node1 := kadid.NewKadID([]byte{0x80})
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint(node0, sched0, router)
	fendpoint1 := fe.NewFakeEndpoint(node1, sched1, router)
	rt0 := simplert.New(node0.Key(), 1)
	rt1 := simplert.New(node1.Key(), 1)

	// make node1 a server
	server1 := basicserver.NewBasicServer(rt1, fendpoint1)
	fendpoint1.AddRequestHandler(protoID, &sm.SimMessage{}, server1.HandleRequest)

	// connect add node1 address to node0
	err := fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)
	require.NoError(t, err)
	success, err := rt0.AddPeer(ctx, node1)
	require.NoError(t, err)
	require.True(t, success)

	// add a peer in node1's routing table. this peer will be returned in the
	// response to the query
	node2 := kadid.NewKadID([]byte{0xf0})
	err = fendpoint1.MaybeAddToPeerstore(ctx, node2, peerstoreTTL)
	require.NoError(t, err)
	success, err = rt1.AddPeer(ctx, node2)
	require.NoError(t, err)
	require.True(t, success)

	req := sm.NewSimRequest([]byte{0xf0})

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
	q, err := NewSimpleQuery(ctx, req, queryOpts...)
	require.NoError(t, err)

	// create and run the simulation
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, sched0, sched1)
	sim.Run(ctx)

	// check that the peerlist should contain node2 and node1 (in this order)
	require.Equal(t, node2, q.peerlist.closest.id)
	require.Equal(t, unreachable, q.peerlist.closest.status)
	// node2 is considered unreachable, and not added to the routing table,
	// because it doesn't return any peer (as its routing table is empty)
	require.Equal(t, node1, q.peerlist.closest.next.id)
	// node1 is set as queried because it answer with closer peers (only 1)
	require.Equal(t, queried, q.peerlist.closest.next.status)
	// there are no more peers in the peerlist
	require.Nil(t, q.peerlist.closest.next.next)
	require.Nil(t, q.peerlist.closestQueued)
}

func TestInvalidQueryOptions(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	router := fe.NewFakeRouter()
	node := si.StringID("node0")
	sched := ss.NewSimpleScheduler(clk)
	fendpoint := fe.NewFakeEndpoint(node, sched, router)
	rt := simplert.New(node.Key(), 1)

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

	// fails because req.Target() isn't the same size as node.Key()
	invalidOpts = []Option{
		WithRoutingTable(rt),
		WithEndpoint(fendpoint),
		WithScheduler(sched),
		WithConcurrency(1),
	}
	_, err = NewSimpleQuery(ctx, req, invalidOpts...)
	require.Error(t, err)
}

func simulationSetup(t *testing.T, ctx context.Context, n, bucketSize int,
	clk clock.Clock, protoID address.ProtocolID, peerstoreTTL time.Duration,
	defaultQueryOpts []Option) (
	[]address.NodeAddr, []scheduler.AwareScheduler, []endpoint.SimEndpoint,
	[]routing.Table, []server.Server, [][]Option) {

	router := fe.NewFakeRouter()

	ids := make([]address.NodeAddr, n)
	scheds := make([]scheduler.AwareScheduler, n)
	fendpoints := make([]endpoint.SimEndpoint, n)
	rts := make([]routing.Table, n)
	servers := make([]server.Server, n)

	spacing := 256 / n

	for i := 0; i < n; i++ {
		scheds[i] = ss.NewSimpleScheduler(clk)
		ids[i] = kadid.NewKadID([]byte{byte(i * spacing)})
		fendpoints[i] = fe.NewFakeEndpoint(ids[i].NodeID(), scheds[i], router)
		rts[i] = simplert.New(ids[i].NodeID().Key(), bucketSize)
		servers[i] = basicserver.NewBasicServer(rts[i], fendpoints[i],
			basicserver.WithNumberUsefulCloserPeers(bucketSize))
		fendpoints[i].AddRequestHandler(protoID, &sm.SimMessage{}, servers[i].HandleRequest)
	}

	// peer ids (KadIDs) are i*8 for i in [0, 32), the keyspace is 1 byte [0, 255]
	// so peers cover the whole keyspace, and they have the format XXXX X000.

	// connect peers, and add them to the routing tables
	// note: when the bucket is complete, it contains the peers with the
	// smallest identifier (KadID).
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// add peer to peerstore
			err := fendpoints[i].MaybeAddToPeerstore(ctx, ids[j], peerstoreTTL)
			require.NoError(t, err)
			// we don't require the the peer is added to the routing table,
			// because the bucket might be full already and it is fine
			_, err = rts[i].AddPeer(ctx, ids[j].NodeID())
			require.NoError(t, err)
		}
	}

	// query options for each peer
	queryOpts := make([][]Option, n)
	for i := 0; i < n; i++ {
		queryOpts[i] = append(defaultQueryOpts,
			WithRoutingTable(rts[i]),
			WithEndpoint(fendpoints[i]),
			WithScheduler(scheds[i]),
		)
	}

	return ids, scheds, fendpoints, rts, servers, queryOpts
}

func getHandleResults(t *testing.T, req message.MinKadRequestMessage,
	expectedPeers []key.KadKey, expectedResponses [][]key.KadKey) func(
	ctx context.Context, id address.NodeID, resp message.MinKadResponseMessage) (
	bool, []address.NodeID) {

	var responseCount int
	return func(ctx context.Context, id address.NodeID,
		resp message.MinKadResponseMessage) (bool, []address.NodeID) {
		// check that the request was sent to the correct peer
		require.Equal(t, expectedPeers[responseCount], id.Key(), "responseCount: ", responseCount)

		ids := make([]address.NodeID, len(resp.CloserNodes()))
		var found bool
		for i, n := range resp.CloserNodes() {
			ids[i] = n.NodeID()
			if match, err := ids[i].Key().Equal(req.Target()); err == nil && match {
				// the target was found, stop the query
				found = true
			}
			// check that the response contains the expected peers
			require.Contains(t, expectedResponses[responseCount], n.NodeID().Key())
		}
		responseCount++
		return found, ids
	}
}

func TestElementaryQuery(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	bucketSize := 4
	nPeers := 32
	peerstoreTTL := time.Minute

	// generic query options to be used by all peers
	defaultQueryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(1),
		WithNumberUsefulCloserPeers(bucketSize),
		WithRequestTimeout(time.Second),
		WithPeerstoreTTL(peerstoreTTL),
	}

	ids, scheds, _, rts, _, queryOpts := simulationSetup(t, ctx, nPeers,
		bucketSize, clk, protoID, peerstoreTTL, defaultQueryOpts)

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sm.NewSimRequest(ids[len(ids)-1].NodeID().Key())

	// peers that are expected to be queried, in order
	expectedPeers := []key.KadKey{}
	// peer that are expected to be included in responses, in order
	expectedResponses := [][]key.KadKey{}

	currID := 0
	// while currID != target.Key()
	for c, _ := ids[currID].NodeID().Key().Equal(req.Target()); !c; {
		// get closest peer to target from the sollicited peer
		closest, err := rts[currID].NearestPeers(ctx, req.Target(), 1)
		require.NoError(t, err)
		require.Len(t, closest, 1, fmt.Sprint(ids[currID].NodeID().Key().Equal(req.Target())))
		expectedPeers = append(expectedPeers, closest[0].Key())

		// the next current id is the closest peer to the target
		currID = int(closest[0].Key()[0] / 8)

		// the peers included in the response are the closest to the target
		// from the sollicited peer
		responseClosest, err := rts[currID].NearestPeers(ctx, req.Target(), bucketSize)
		require.NoError(t, err)
		closestKey := make([]key.KadKey, len(responseClosest))
		for i, n := range responseClosest {
			closestKey[i] = n.Key()
		}
		expectedResponses = append(expectedResponses, closestKey)

		// test if the current ID is the target
		c, _ = ids[currID].NodeID().Key().Equal(req.Target())
	}

	// handleResults is called when a peer receives a response from a peer. If
	// the response contains the target, it returns true, and the query stops.
	// Otherwise, it returns false, and the query continues. This function also
	// checks that the response come from the expected peer and contains the
	// expected peers addresses.
	handleResults := getHandleResults(t, req, expectedPeers, expectedResponses)

	// the request will not fail
	notifyFailure := func(context.Context) {
		require.Fail(t, "notify failure shouldn't be called")
	}

	_, err := NewSimpleQuery(ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc(notifyFailure))...)
	require.NoError(t, err)

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, scheds...)
	// run simulation
	sim.Run(ctx)
}

func TestFailedQuery(t *testing.T) {
	// the key doesn't exist and cannot be found, test with no exit condition
	// all peers of the peerlist should have been queried by the end

	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	bucketSize := 4
	nPeers := 16
	peerstoreTTL := time.Minute

	// generic query options to be used by all peers
	defaultQueryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(1),
		WithNumberUsefulCloserPeers(bucketSize),
		WithRequestTimeout(time.Second),
		WithPeerstoreTTL(peerstoreTTL),
	}

	ids, scheds, _, rts, _, queryOpts := simulationSetup(t, ctx, nPeers,
		bucketSize, clk, protoID, peerstoreTTL, defaultQueryOpts)

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sm.NewSimRequest([]byte{0xff})

	//         _______^_______
	//      __^__           __^__
	//     /     \         /     \
	//   / \     / \     / \     / \
	// / \ / \ / \ / \ / \ / \ / \ / \
	// 0 1 2 3 4 5 6 7 8 9 a b c d e f
	//                 * * * *          // closest peers to 0xff in 0's routing
	//                                  // table, closest is b
	//
	//                         * * * *  // closest peers to 0xff in b's routing
	//                                  // table, closest is f
	//
	// - 0 will add b, a, 9, 8 to the query's peerlist.
	// - 0 queries b first, that respond with closer peers to 0xff: f, e, d, c
	// - then 0 won't learn any more new peers, and query all the peerlist from
	// the closest to the furthest from 0xff: f, e, d, c, a, 9, 8.
	// - note that the closest peers to 0xff for 8, 9, a, b, c, d, e, f are
	// always [f, e, d, c].

	order := []int{0xb0, 0xf0, 0xe0, 0xd0, 0xc0, 0xa0, 0x90, 0x80}
	for i, o := range order {
		// 16 is the spacing between the nodes
		order[i] = o / 16
	}

	// peers that are expected to be queried, in order
	expectedPeers := make([]key.KadKey, len(order))
	for i, o := range order {
		expectedPeers[i] = ids[o].NodeID().Key()
	}

	// peer that are expected to be included in responses, in order
	expectedResponses := make([][]key.KadKey, len(order))
	for i, o := range order {
		// the peers included in the response are the closest to the target
		// from the sollicited peer
		responseClosest, err := rts[o].NearestPeers(ctx, req.Target(), bucketSize)
		require.NoError(t, err)
		closestKeys := make([]key.KadKey, len(responseClosest))
		for i, n := range responseClosest {
			closestKeys[i] = n.Key()
		}
		expectedResponses[i] = closestKeys
	}

	// handleResults is called when a peer receives a response from a peer. If
	// the response contains the target, it returns true, and the query stops.
	// Otherwise, it returns false, and the query continues. This function also
	// checks that the response come from the expected peer and contains the
	// expected peers addresses.
	handleResults := getHandleResults(t, req, expectedPeers, expectedResponses)

	var failed bool
	// the request will not fail
	notifyFailure := func(context.Context) {
		failed = true
	}

	_, err := NewSimpleQuery(ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc(notifyFailure))...)
	require.NoError(t, err)

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, scheds...)
	// run simulation
	sim.Run(ctx)

	require.True(t, failed)
}

func TestConcurrentQuery(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	bucketSize := 4
	nPeers := 8
	peerstoreTTL := time.Minute

	router := fe.NewFakeRouter()

	ids := make([]address.NodeAddr, nPeers)
	scheds := make([]scheduler.AwareScheduler, nPeers)
	fendpoints := make([]endpoint.SimEndpoint, nPeers)
	rts := make([]routing.Table, nPeers)
	servers := make([]server.Server, nPeers)

	for i := 0; i < nPeers; i++ {
		scheds[i] = ss.NewSimpleScheduler(clk)
		ids[i] = kadid.NewKadID([]byte{byte(i * 32)})
		fendpoints[i] = fe.NewFakeEndpoint(ids[i].NodeID(), scheds[i], router)
		rts[i] = simplert.New(ids[i].NodeID().Key(), bucketSize)
		servers[i] = basicserver.NewBasicServer(rts[i], fendpoints[i],
			basicserver.WithNumberUsefulCloserPeers(bucketSize))
		fendpoints[i].AddRequestHandler(protoID, &sm.SimMessage{}, servers[i].HandleRequest)
	}

	// 0 is looking for 7
	// 0 knows 1, 2, 3: it will query 3, 2 at first
	// 3 knows 4, 0
	// 2 knows 6, 0
	// 4 knows 5, 3
	// 6 knows 7, 2, 5
	// 5 knows 4, 6
	// sequence of outgoing requests from 0 to find 7: 3, 2, 4, 6

	connections := [...][2]int{{0, 1}, {0, 2}, {0, 3}, {3, 4}, {2, 6},
		{4, 5}, {6, 7}, {6, 5}}

	for _, c := range connections {
		for i := range c {
			// add peer to peerstore
			err := fendpoints[c[i]].MaybeAddToPeerstore(ctx, ids[c[1-i]], peerstoreTTL)
			require.NoError(t, err)
			// we don't require the the peer is added to the routing table,
			// because the bucket might be full already and it is fine
			_, err = rts[c[i]].AddPeer(ctx, ids[c[1-i]].NodeID())
			require.NoError(t, err)
		}
	}

	// generic query options to be used by all peers
	defaultQueryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(2),
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

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sm.NewSimRequest(ids[len(ids)-1].NodeID().Key())

	// peers that are expected to be queried, in order
	expectedPeers := []key.KadKey{ids[3].NodeID().Key(), ids[2].NodeID().Key(),
		ids[4].NodeID().Key(), ids[6].NodeID().Key()}
	// peer that are expected to be included in responses, in order
	expectedResponses := [][]key.KadKey{
		{ids[4].NodeID().Key(), ids[0].NodeID().Key()},
		{ids[6].NodeID().Key(), ids[0].NodeID().Key()},
		{ids[5].NodeID().Key(), ids[3].NodeID().Key()},
		{ids[7].NodeID().Key(), ids[2].NodeID().Key(), ids[5].NodeID().Key()},
	}

	handleResults := getHandleResults(t, req, expectedPeers, expectedResponses)

	// the request will not fail
	notifyFailure := func(context.Context) {
		require.Fail(t, "notify failure shouldn't be called")
	}

	_, err := NewSimpleQuery(ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc(notifyFailure))...)
	require.NoError(t, err)

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, scheds...)
	// run simulation
	sim.Run(ctx)
}

func TestUnresponsivePeer(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	peerstoreTTL := time.Minute
	bucketSize := 1

	router := fe.NewFakeRouter()
	id0 := kadid.NewKadID([]byte{0})
	id1 := kadid.NewKadID([]byte{1})
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint(id0.NodeID(), sched0, router)
	fendpoint1 := fe.NewFakeEndpoint(id1.NodeID(), sched1, router)
	rt0 := simplert.New(id0.NodeID().Key(), bucketSize)

	serverRequestHandler := func(context.Context, address.NodeID,
		message.MinKadMessage) (message.MinKadMessage, error) {
		return nil, errors.New("")
	}
	fendpoint1.AddRequestHandler(protoID, &sm.SimMessage{}, serverRequestHandler)

	req := sm.NewSimRequest([]byte{0xff})

	responseHandler := func(ctx context.Context, sender address.NodeID,
		msg message.MinKadResponseMessage) (bool, []address.NodeID) {
		require.Fail(t, "response handler shouldn't be called")
		return false, nil
	}
	queryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(2),
		WithNumberUsefulCloserPeers(bucketSize),
		WithRequestTimeout(time.Millisecond),
		WithEndpoint(fendpoint0),
		WithRoutingTable(rt0),
		WithScheduler(sched0),
		WithHandleResultsFunc(responseHandler),
	}

	// query creation fails because the routing table of 0 is empty
	_, err := NewSimpleQuery(ctx, req, queryOpts...)
	require.Error(t, err)

	// connect 0 and 1
	err = fendpoint0.MaybeAddToPeerstore(ctx, id1, peerstoreTTL)
	require.NoError(t, err)
	success, err := rt0.AddPeer(ctx, id1.NodeID())
	require.NoError(t, err)
	require.True(t, success)

	q, err := NewSimpleQuery(ctx, req, queryOpts...)
	require.NoError(t, err)

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, sched0, sched1)
	// run simulation
	sim.Run(ctx)

	// make sure the peer is marked as unreachable
	require.Equal(t, unreachable, q.peerlist.closest.status)
}

func TestCornerCases(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")
	bucketSize := 1
	peerstoreTTL := time.Minute

	router := fe.NewFakeRouter()
	id0 := kadid.NewKadID([]byte{0x00})
	sched0 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint(id0.NodeID(), sched0, router)
	rt0 := simplert.New(id0.NodeID().Key(), bucketSize)

	id1 := kadid.NewKadID([]byte{0x01})
	fendpoint0.MaybeAddToPeerstore(ctx, id1, peerstoreTTL)

	success, err := rt0.AddPeer(ctx, id1)
	require.NoError(t, err)
	require.True(t, success)

	req := sm.NewSimRequest([]byte{0xff})

	responseHandler := func(ctx context.Context, sender address.NodeID,
		msg message.MinKadResponseMessage) (bool, []address.NodeID) {
		ids := make([]address.NodeID, len(msg.CloserNodes()))
		for i, peer := range msg.CloserNodes() {
			ids[i] = peer.NodeID()
		}
		return false, ids
	}

	queryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(1),
		WithNumberUsefulCloserPeers(bucketSize),
		WithRequestTimeout(time.Millisecond),
		WithEndpoint(fendpoint0),
		WithRoutingTable(rt0),
		WithScheduler(sched0),
		WithHandleResultsFunc(responseHandler),
	}

	q, err := NewSimpleQuery(ctx, req, queryOpts...)
	require.NoError(t, err)

	// nil response should trigger a request error
	q.handleResponse(ctx, id1, nil)

	addrs := []address.NodeAddr{
		kadaddr.NewKadAddr(kadid.NewKadID([]byte{0xee}), nil),
		kadaddr.NewKadAddr(kadid.NewKadID([]byte{0x66, 0x66}), nil), // invalid key length
		kadaddr.NewKadAddr(kadid.NewKadID([]byte{0x88}), nil),
	}
	forgedResponse := sm.NewSimResponse(addrs)
	q.handleResponse(ctx, id1, forgedResponse)

	// test that 0xee and 0x88 have been added to peerlist but not 0x6666
	require.Equal(t, addrs[0].NodeID(), q.peerlist.closest.id)
	require.Equal(t, addrs[2].NodeID(), q.peerlist.closest.next.id)
	require.Equal(t, id1, q.peerlist.closest.next.next.id)
	require.Nil(t, q.peerlist.closest.next.next.next)

	// test new request if all peers have been tried
	for q.peerlist.closestQueued != nil {
		q.peerlist.popClosestQueued()
	}
	q.inflightRequests = 1
	q.newRequest(ctx)
	require.Equal(t, 0, q.inflightRequests)

	// cancel contex
	cancel()
	sched0.EnqueueAction(ctx, basicaction.BasicAction(func(context.Context) {
		q.requestError(ctx, id1, errors.New(""))
	}))

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, sched0)
	// run simulation
	sim.Run(ctx)

	require.True(t, q.done)
}

// TestLibp2pCornerCase tests that the newRequest(ctx) can fail fast if the
// selected peer has an invalid format (e.g for libp2p something that isn't a
// peerid.PeerID). This test should be performed using the libp2p endpoint
// because the fakeendpoint can never fail fast.
func TestLibp2pCornerCase(t *testing.T) {
	ctx := context.Background()
	clk := clock.New()

	protoID := address.ProtocolID("/test/1.0.0")
	bucketSize := 1
	peerstoreTTL := time.Minute

	h, err := libp2p.New()
	require.NoError(t, err)
	id := peerid.NewPeerID(h.ID())
	sched := ss.NewSimpleScheduler(clk)
	libp2pEndpoint := libp2pendpoint.NewLibp2pEndpoint(ctx, h, sched)
	rt := simplert.New(id.Key(), bucketSize)

	parsed, err := peer.Decode("1D3oooUnknownPeer")
	require.NoError(t, err)
	addrInfo := addrinfo.NewAddrInfo(peer.AddrInfo{
		ID:    parsed,
		Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/1.1.1.1/tcp/1")},
	})
	success, err := rt.AddPeer(ctx, addrInfo.NodeID())
	require.NoError(t, err)
	require.True(t, success)
	err = libp2pEndpoint.MaybeAddToPeerstore(ctx, addrInfo, peerstoreTTL)
	require.NoError(t, err)

	queryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(1),
		WithNumberUsefulCloserPeers(bucketSize),
		WithRequestTimeout(time.Millisecond),
		WithEndpoint(libp2pEndpoint),
		WithRoutingTable(rt),
		WithScheduler(sched),
	}

	req := sm.NewSimRequest(si.NewStringID("RandomKey").Key())

	q2, err := NewSimpleQuery(ctx, req, queryOpts...)
	require.NoError(t, err)

	// set the peerlist endpoint to nil, to allow invalid NodeIDs in peerlist
	q2.peerlist.endpoint = nil
	// change the node id of the queued peer. sending the message with
	// SendRequestHandleResponse will fail fast (no new go routine created)
	q2.peerlist.closest.id = kadid.NewKadID([]byte{0x00})

	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))
}
