package simplequery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/plprobelab/go-kademlia/events/action/basicaction"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	ss "github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/events/simulator"
	"github.com/plprobelab/go-kademlia/events/simulator/litesimulator"
	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/server"
	"github.com/plprobelab/go-kademlia/server/basicserver"
	"github.com/plprobelab/go-kademlia/sim"
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

	router := sim.NewRouter[key.Key256, any]()
	node0 := kadtest.NewAddr[key.Key256, any](kadtest.NewID(key.ZeroKey256()), nil)
	node1 := kadtest.NewAddr[key.Key256, any](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0x80})), nil)
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := sim.NewEndpoint[key.Key256, any](node0.ID(), sched0, router)
	fendpoint1 := sim.NewEndpoint[key.Key256, any](node1.ID(), sched1, router)
	rt0 := simplert.New[key.Key256, any](node0.ID().Key(), 1)
	rt1 := simplert.New[key.Key256, any](node1.ID().Key(), 1)

	// make node1 a server
	server1 := basicserver.NewBasicServer[key.Key256, any](rt1, fendpoint1)
	fendpoint1.AddRequestHandler(protoID, &sim.Message[key.Key256, any]{}, server1.HandleRequest)

	// connect add node1 address to node0
	err := fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)
	require.NoError(t, err)
	success := rt0.AddNode(node1.ID())
	require.True(t, success)

	// add a peer in node1's routing table. this peer will be returned in the
	// response to the query
	node2 := kadtest.NewAddr[key.Key256, any](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{0xf0})), nil)
	err = fendpoint1.MaybeAddToPeerstore(ctx, node2, peerstoreTTL)
	require.NoError(t, err)
	success = rt1.AddNode(node2.ID())
	require.True(t, success)

	req := sim.NewRequest[key.Key256, any](kadtest.Key256WithLeadingBytes([]byte{0xf0}))

	queryOpts := []Option[key.Key256, any]{
		WithProtocolID[key.Key256, any](protoID),
		WithConcurrency[key.Key256, any](1),
		WithNumberUsefulCloserPeers[key.Key256, any](2),
		WithRequestTimeout[key.Key256, any](time.Second),
		WithPeerstoreTTL[key.Key256, any](peerstoreTTL),
		WithRoutingTable[key.Key256, any](rt0),
		WithEndpoint[key.Key256, any](fendpoint0),
		WithScheduler[key.Key256, any](sched0),
	}
	q, err := NewSimpleQuery[key.Key256, any](ctx, req, queryOpts...)
	require.NoError(t, err)

	// create and run the simulation
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, sched0, sched1)
	sim.Run(ctx)

	// check that the peerlist should contain node2 and node1 (in this order)
	require.Equal(t, node2.ID(), q.peerlist.closest.id)
	require.Equal(t, unreachable, q.peerlist.closest.status)
	// node2 is considered unreachable, and not added to the routing table,
	// because it doesn't return any peer (as its routing table is empty)
	require.Equal(t, node1.ID(), q.peerlist.closest.next.id)
	// node1 is set as queried because it answer with closer peers (only 1)
	require.Equal(t, queried, q.peerlist.closest.next.status)
	// there are no more peers in the peerlist
	require.Nil(t, q.peerlist.closest.next.next)
	require.Nil(t, q.peerlist.closestQueued)
}

func TestInvalidQueryOptions(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	router := sim.NewRouter[key.Key256, any]()
	node := kadtest.StringID("node0")
	sched := ss.NewSimpleScheduler(clk)
	fendpoint := sim.NewEndpoint[key.Key256, any](node, sched, router)
	rt := simplert.New[key.Key256, any](node.Key(), 1)

	// fails because rt is not set
	invalidOpts := []Option[key.Key256, any]{}
	req := sim.NewRequest[key.Key256, any](key.ZeroKey256())
	_, err := NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because rt is nil
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](nil),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because endpoint is not set
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because endpoint is nil
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
		WithEndpoint[key.Key256, any](nil),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because scheduler is not set
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
		WithEndpoint[key.Key256, any](fendpoint),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because scheduler is nil
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
		WithEndpoint[key.Key256, any](fendpoint),
		WithScheduler[key.Key256, any](nil),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because HandleResultsFunc is nil
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
		WithEndpoint[key.Key256, any](fendpoint),
		WithScheduler[key.Key256, any](sched),
		WithHandleResultsFunc[key.Key256, any](nil),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because WithNotifyFailureFunc is nil
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
		WithEndpoint[key.Key256, any](fendpoint),
		WithScheduler[key.Key256, any](sched),
		WithNotifyFailureFunc[key.Key256, any](nil),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because NumberUsefulCloserPeers is 0
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
		WithEndpoint[key.Key256, any](fendpoint),
		WithScheduler[key.Key256, any](sched),
		WithNumberUsefulCloserPeers[key.Key256, any](0),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because Concurrency is 0
	invalidOpts = []Option[key.Key256, any]{
		WithRoutingTable[key.Key256, any](rt),
		WithEndpoint[key.Key256, any](fendpoint),
		WithScheduler[key.Key256, any](sched),
		WithConcurrency[key.Key256, any](0),
	}
	_, err = NewSimpleQuery[key.Key256, any](ctx, req, invalidOpts...)
	require.Error(t, err)
}

func simulationSetup(t *testing.T, ctx context.Context, n, bucketSize int,
	clk clock.Clock, protoID address.ProtocolID, peerstoreTTL time.Duration,
	defaultQueryOpts []Option[key.Key8, any]) (
	[]kad.NodeInfo[key.Key8, any], []scheduler.AwareScheduler, []endpoint.SimEndpoint[key.Key8, any],
	[]kad.RoutingTable[key.Key8], []server.Server[key.Key8], [][]Option[key.Key8, any],
) {
	router := sim.NewRouter[key.Key8, any]()

	ids := make([]kad.NodeInfo[key.Key8, any], n)
	scheds := make([]scheduler.AwareScheduler, n)
	fendpoints := make([]endpoint.SimEndpoint[key.Key8, any], n)
	rts := make([]kad.RoutingTable[key.Key8], n)
	servers := make([]server.Server[key.Key8], n)

	spacing := 256 / n

	for i := 0; i < n; i++ {
		scheds[i] = ss.NewSimpleScheduler(clk)
		ids[i] = kadtest.NewAddr[key.Key8, any](kadtest.NewID(key.Key8(uint8(i*spacing))), nil)
		fendpoints[i] = sim.NewEndpoint[key.Key8, any](ids[i].ID(), scheds[i], router)
		rts[i] = simplert.New[key.Key8, any](ids[i].ID().Key(), bucketSize)
		cfg := sim.DefaultServerConfig()
		cfg.NumberUsefulCloserPeers = bucketSize
		servers[i] = sim.NewServer[key.Key8, any](rts[i], fendpoints[i], cfg)
		fendpoints[i].AddRequestHandler(protoID, &sim.Message[key.Key8, any]{}, servers[i].HandleRequest)
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
			rts[i].AddNode(ids[j].ID())
		}
	}

	// query options for each peer
	queryOpts := make([][]Option[key.Key8, any], n)
	for i := 0; i < n; i++ {
		queryOpts[i] = append(defaultQueryOpts,
			WithRoutingTable[key.Key8, any](rts[i]),
			WithEndpoint[key.Key8, any](fendpoints[i]),
			WithScheduler[key.Key8, any](scheds[i]),
		)
	}

	return ids, scheds, fendpoints, rts, servers, queryOpts
}

func getHandleResults[K kad.Key[K], A any](t *testing.T, req message.MinKadRequestMessage[K, A],
	expectedPeers []K, expectedResponses [][]K) func(
	ctx context.Context, id kad.NodeID[K], resp message.MinKadResponseMessage[K, A]) (
	bool, []kad.NodeID[K]) {
	var responseCount int
	return func(ctx context.Context, id kad.NodeID[K],
		resp message.MinKadResponseMessage[K, A],
	) (bool, []kad.NodeID[K]) {
		// check that the request was sent to the correct peer
		require.Equal(t, expectedPeers[responseCount], id.Key(), "responseCount: ", responseCount)

		ids := make([]kad.NodeID[K], len(resp.CloserNodes()))
		var found bool
		for i, n := range resp.CloserNodes() {
			ids[i] = n.ID()
			if key.Equal(ids[i].Key(), req.Target()) {
				// the target was found, stop the query
				found = true
			}
			// check that the response contains the expected peers
			require.Contains(t, expectedResponses[responseCount], n.ID().Key())
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
	defaultQueryOpts := []Option[key.Key8, any]{
		WithProtocolID[key.Key8, any](protoID),
		WithConcurrency[key.Key8, any](1),
		WithNumberUsefulCloserPeers[key.Key8, any](bucketSize),
		WithRequestTimeout[key.Key8, any](time.Second),
		WithPeerstoreTTL[key.Key8, any](peerstoreTTL),
	}

	ids, scheds, _, rts, _, queryOpts := simulationSetup(t, ctx, nPeers,
		bucketSize, clk, protoID, peerstoreTTL, defaultQueryOpts)

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sim.NewRequest[key.Key8, any](ids[len(ids)-1].ID().Key())

	// peers that are expected to be queried, in order
	expectedPeers := []key.Key8{}
	// peer that are expected to be included in responses, in order
	expectedResponses := [][]key.Key8{}

	currID := 0
	// while currID != target.Key()
	for !key.Equal(ids[currID].ID().Key(), req.Target()) {
		// get closest peer to target from the sollicited peer
		closest := rts[currID].NearestNodes(req.Target(), 1)
		require.Len(t, closest, 1, fmt.Sprint(key.Equal(ids[currID].ID().Key(), req.Target())))
		expectedPeers = append(expectedPeers, closest[0].Key())

		// the next current id is the closest peer to the target
		currID = int(closest[0].Key() / 8)

		// the peers included in the response are the closest to the target
		// from the sollicited peer
		responseClosest := rts[currID].NearestNodes(req.Target(), bucketSize)
		closestKey := make([]key.Key8, len(responseClosest))
		for i, n := range responseClosest {
			closestKey[i] = n.Key()
		}
		expectedResponses = append(expectedResponses, closestKey)
	}

	// handleResults is called when a peer receives a response from a peer. If
	// the response contains the target, it returns true, and the query stops.
	// Otherwise, it returns false, and the query continues. This function also
	// checks that the response come from the expected peer and contains the
	// expected peers addresses.
	handleResults := getHandleResults[key.Key8, any](t, req, expectedPeers, expectedResponses)

	// the request will not fail
	notifyFailure := func(context.Context) {
		require.Fail(t, "notify failure shouldn't be called")
	}

	_, err := NewSimpleQuery[key.Key8, any](ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc[key.Key8, any](notifyFailure))...)
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
	defaultQueryOpts := []Option[key.Key8, any]{
		WithProtocolID[key.Key8, any](protoID),
		WithConcurrency[key.Key8, any](1),
		WithNumberUsefulCloserPeers[key.Key8, any](bucketSize),
		WithRequestTimeout[key.Key8, any](time.Second),
		WithPeerstoreTTL[key.Key8, any](peerstoreTTL),
	}

	ids, scheds, _, rts, _, queryOpts := simulationSetup(t, ctx, nPeers,
		bucketSize, clk, protoID, peerstoreTTL, defaultQueryOpts)

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sim.NewRequest[key.Key8, any](key.Key8(0xff))

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
	expectedPeers := make([]key.Key8, len(order))
	for i, o := range order {
		expectedPeers[i] = ids[o].ID().Key()
	}

	// peer that are expected to be included in responses, in order
	expectedResponses := make([][]key.Key8, len(order))
	for i, o := range order {
		// the peers included in the response are the closest to the target
		// from the sollicited peer
		responseClosest := rts[o].NearestNodes(req.Target(), bucketSize)
		closestKeys := make([]key.Key8, len(responseClosest))
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
	handleResults := getHandleResults[key.Key8, any](t, req, expectedPeers, expectedResponses)

	var failed bool
	// the request will not fail
	notifyFailure := func(context.Context) {
		failed = true
	}

	_, err := NewSimpleQuery[key.Key8, any](ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc[key.Key8, any](notifyFailure))...)
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

	router := sim.NewRouter[key.Key256, any]()

	ids := make([]kad.NodeInfo[key.Key256, any], nPeers)
	scheds := make([]scheduler.AwareScheduler, nPeers)
	fendpoints := make([]endpoint.SimEndpoint[key.Key256, any], nPeers)
	rts := make([]kad.RoutingTable[key.Key256], nPeers)
	servers := make([]server.Server[key.Key256], nPeers)

	for i := 0; i < nPeers; i++ {
		scheds[i] = ss.NewSimpleScheduler(clk)
		ids[i] = kadtest.NewAddr[key.Key256, any](kadtest.NewID(kadtest.Key256WithLeadingBytes([]byte{byte(i * 32)})), nil)
		fendpoints[i] = sim.NewEndpoint(ids[i].ID(), scheds[i], router)
		rts[i] = simplert.New[key.Key256, any](ids[i].ID().Key(), bucketSize)
		servers[i] = basicserver.NewBasicServer[key.Key256, any](rts[i], fendpoints[i],
			basicserver.WithNumberUsefulCloserPeers(bucketSize))
		fendpoints[i].AddRequestHandler(protoID, &sim.Message[key.Key256, any]{}, servers[i].HandleRequest)
	}

	// 0 is looking for 7
	// 0 knows 1, 2, 3: it will query 3, 2 at first
	// 3 knows 4, 0
	// 2 knows 6, 0
	// 4 knows 5, 3
	// 6 knows 7, 2, 5
	// 5 knows 4, 6
	// sequence of outgoing requests from 0 to find 7: 3, 2, 4, 6

	connections := [...][2]int{
		{0, 1},
		{0, 2},
		{0, 3},
		{3, 4},
		{2, 6},
		{4, 5},
		{6, 7},
		{6, 5},
	}

	for _, c := range connections {
		for i := range c {
			// add peer to peerstore
			err := fendpoints[c[i]].MaybeAddToPeerstore(ctx, ids[c[1-i]], peerstoreTTL)
			require.NoError(t, err)
			// we don't require the the peer is added to the routing table,
			// because the bucket might be full already and it is fine
			rts[c[i]].AddNode(ids[c[1-i]].ID())
		}
	}

	// generic query options to be used by all peers
	defaultQueryOpts := []Option[key.Key256, any]{
		WithProtocolID[key.Key256, any](protoID),
		WithConcurrency[key.Key256, any](2),
		WithNumberUsefulCloserPeers[key.Key256, any](bucketSize),
		WithRequestTimeout[key.Key256, any](time.Second),
		WithPeerstoreTTL[key.Key256, any](peerstoreTTL),
	}

	// query options for each peer
	queryOpts := make([][]Option[key.Key256, any], nPeers)
	for i := 0; i < nPeers; i++ {
		queryOpts[i] = append(defaultQueryOpts,
			WithRoutingTable[key.Key256, any](rts[i]),
			WithEndpoint[key.Key256, any](fendpoints[i]),
			WithScheduler[key.Key256, any](scheds[i]),
		)
	}

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sim.NewRequest[key.Key256, any](ids[len(ids)-1].ID().Key())

	// peers that are expected to be queried, in order
	expectedPeers := []key.Key256{
		ids[3].ID().Key(), ids[2].ID().Key(),
		ids[4].ID().Key(), ids[6].ID().Key(),
	}
	// peer that are expected to be included in responses, in order
	expectedResponses := [][]key.Key256{
		{ids[4].ID().Key(), ids[0].ID().Key()},
		{ids[6].ID().Key(), ids[0].ID().Key()},
		{ids[5].ID().Key(), ids[3].ID().Key()},
		{ids[7].ID().Key(), ids[2].ID().Key(), ids[5].ID().Key()},
	}

	handleResults := getHandleResults[key.Key256, any](t, req, expectedPeers, expectedResponses)

	// the request will not fail
	notifyFailure := func(context.Context) {
		require.Fail(t, "notify failure shouldn't be called")
	}

	_, err := NewSimpleQuery[key.Key256, any](ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc[key.Key256, any](notifyFailure))...)
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

	router := sim.NewRouter[key.Key8, any]()
	node0 := kadtest.NewAddr[key.Key8, any](kadtest.NewID(key.Key8(0)), nil)
	node1 := kadtest.NewAddr[key.Key8, any](kadtest.NewID(key.Key8(1)), nil)
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := sim.NewEndpoint[key.Key8, any](node0.ID(), sched0, router)
	fendpoint1 := sim.NewEndpoint[key.Key8, any](node1.ID(), sched1, router)
	rt0 := simplert.New[key.Key8, any](node0.ID().Key(), bucketSize)

	serverRequestHandler := func(context.Context, kad.NodeID[key.Key8],
		message.MinKadMessage,
	) (message.MinKadMessage, error) {
		return nil, errors.New("")
	}
	fendpoint1.AddRequestHandler(protoID, &sim.Message[key.Key8, any]{}, serverRequestHandler)

	req := sim.NewRequest[key.Key8, any](key.Key8(0xff))

	responseHandler := func(ctx context.Context, sender kad.NodeID[key.Key8],
		msg message.MinKadResponseMessage[key.Key8, any],
	) (bool, []kad.NodeID[key.Key8]) {
		require.Fail(t, "response handler shouldn't be called")
		return false, nil
	}
	queryOpts := []Option[key.Key8, any]{
		WithProtocolID[key.Key8, any](protoID),
		WithConcurrency[key.Key8, any](2),
		WithNumberUsefulCloserPeers[key.Key8, any](bucketSize),
		WithRequestTimeout[key.Key8, any](time.Millisecond),
		WithEndpoint[key.Key8, any](fendpoint0),
		WithRoutingTable[key.Key8, any](rt0),
		WithScheduler[key.Key8, any](sched0),
		WithHandleResultsFunc[key.Key8, any](responseHandler),
	}

	// query creation fails because the routing table of 0 is empty
	_, err := NewSimpleQuery[key.Key8, any](ctx, req, queryOpts...)
	require.Error(t, err)

	// connect 0 and 1
	err = fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)
	require.NoError(t, err)
	success := rt0.AddNode(node1.ID())
	require.True(t, success)

	q, err := NewSimpleQuery[key.Key8, any](ctx, req, queryOpts...)
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

	router := sim.NewRouter[key.Key8, any]()
	node0 := kadtest.NewAddr[key.Key8, any](kadtest.NewID(key.Key8(0x00)), nil)
	sched0 := ss.NewSimpleScheduler(clk)
	fendpoint0 := sim.NewEndpoint(node0.ID(), sched0, router)
	rt0 := simplert.New[key.Key8, any](node0.ID().Key(), bucketSize)

	node1 := kadtest.NewAddr[key.Key8, any](kadtest.NewID(key.Key8(0x01)), nil)
	fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)

	success := rt0.AddNode(node1.ID())
	require.True(t, success)

	req := sim.NewRequest[key.Key8, any](key.Key8(0xff))

	responseHandler := func(ctx context.Context, sender kad.NodeID[key.Key8],
		msg message.MinKadResponseMessage[key.Key8, any],
	) (bool, []kad.NodeID[key.Key8]) {
		ids := make([]kad.NodeID[key.Key8], len(msg.CloserNodes()))
		for i, peer := range msg.CloserNodes() {
			ids[i] = peer.ID()
		}
		return false, ids
	}

	queryOpts := []Option[key.Key8, any]{
		WithProtocolID[key.Key8, any](protoID),
		WithConcurrency[key.Key8, any](1),
		WithNumberUsefulCloserPeers[key.Key8, any](bucketSize),
		WithRequestTimeout[key.Key8, any](time.Millisecond),
		WithEndpoint[key.Key8, any](fendpoint0),
		WithRoutingTable[key.Key8, any](rt0),
		WithScheduler[key.Key8, any](sched0),
		WithHandleResultsFunc(responseHandler),
	}

	q, err := NewSimpleQuery[key.Key8, any](ctx, req, queryOpts...)
	require.NoError(t, err)

	// nil response should trigger a request error
	q.handleResponse(ctx, node1.ID(), nil)

	addrs := []kad.NodeInfo[key.Key8, any]{
		kadtest.NewAddr[key.Key8, any](kadtest.NewID(key.Key8(0xee)), nil),
		// kadtest.NewAddr()(kadtest.NewID()([]byte{0x66, 0x66}), nil), // invalid key length
		kadtest.NewAddr[key.Key8, any](kadtest.NewID(key.Key8(0x88)), nil),
	}
	forgedResponse := sim.NewResponse(addrs)
	q.handleResponse(ctx, node1.ID(), forgedResponse)

	// test that 0xee and 0x88 have been added to peerlist but not 0x6666
	require.Equal(t, addrs[0].ID(), q.peerlist.closest.id)
	require.Equal(t, addrs[1].ID(), q.peerlist.closest.next.id)
	require.Equal(t, node1.ID(), q.peerlist.closest.next.next.id)
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
		q.requestError(ctx, node1.ID(), errors.New(""))
	}))

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, sched0)
	// run simulation
	sim.Run(ctx)

	require.True(t, q.done)
}
