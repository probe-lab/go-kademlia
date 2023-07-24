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
	"github.com/plprobelab/go-kademlia/internal/testutil"
	"github.com/plprobelab/go-kademlia/kad"
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
	"github.com/plprobelab/go-kademlia/server/simserver"
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

	router := fe.NewFakeRouter[key.Key256]()
	node0 := kadaddr.NewKadAddr(kadid.NewKadID(key.ZeroKey256()), nil)
	node1 := kadaddr.NewKadAddr(kadid.NewKadID(testutil.Key256WithLeadingBytes([]byte{0x80})), nil)
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint[key.Key256](node0.NodeID(), sched0, router)
	fendpoint1 := fe.NewFakeEndpoint[key.Key256](node1.NodeID(), sched1, router)
	rt0 := simplert.New(node0.NodeID().Key(), 1)
	rt1 := simplert.New(node1.NodeID().Key(), 1)

	// make node1 a server
	server1 := basicserver.NewBasicServer(rt1, fendpoint1)
	fendpoint1.AddRequestHandler(protoID, &sm.SimMessage[key.Key256]{}, server1.HandleRequest)

	// connect add node1 address to node0
	err := fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)
	require.NoError(t, err)
	success, err := rt0.AddPeer(ctx, node1.NodeID())
	require.NoError(t, err)
	require.True(t, success)

	// add a peer in node1's routing table. this peer will be returned in the
	// response to the query
	node2 := kadaddr.NewKadAddr(kadid.NewKadID(testutil.Key256WithLeadingBytes([]byte{0xf0})), nil)
	err = fendpoint1.MaybeAddToPeerstore(ctx, node2, peerstoreTTL)
	require.NoError(t, err)
	success, err = rt1.AddPeer(ctx, node2.NodeID())
	require.NoError(t, err)
	require.True(t, success)

	req := sm.NewSimRequest(testutil.Key256WithLeadingBytes([]byte{0xf0}))

	queryOpts := []Option[key.Key256]{
		WithProtocolID[key.Key256](protoID),
		WithConcurrency[key.Key256](1),
		WithNumberUsefulCloserPeers[key.Key256](2),
		WithRequestTimeout[key.Key256](time.Second),
		WithPeerstoreTTL[key.Key256](peerstoreTTL),
		WithRoutingTable[key.Key256](rt0),
		WithEndpoint[key.Key256](fendpoint0),
		WithScheduler[key.Key256](sched0),
	}
	q, err := NewSimpleQuery[key.Key256](ctx, req, queryOpts...)
	require.NoError(t, err)

	// create and run the simulation
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, sched0, sched1)
	sim.Run(ctx)

	// check that the peerlist should contain node2 and node1 (in this order)
	require.Equal(t, node2.NodeID(), q.peerlist.closest.id)
	require.Equal(t, unreachable, q.peerlist.closest.status)
	// node2 is considered unreachable, and not added to the routing table,
	// because it doesn't return any peer (as its routing table is empty)
	require.Equal(t, node1.NodeID(), q.peerlist.closest.next.id)
	// node1 is set as queried because it answer with closer peers (only 1)
	require.Equal(t, queried, q.peerlist.closest.next.status)
	// there are no more peers in the peerlist
	require.Nil(t, q.peerlist.closest.next.next)
	require.Nil(t, q.peerlist.closestQueued)
}

func TestInvalidQueryOptions(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	router := fe.NewFakeRouter[key.Key256]()
	node := si.StringID("node0")
	sched := ss.NewSimpleScheduler(clk)
	fendpoint := fe.NewFakeEndpoint[key.Key256](node, sched, router)
	rt := simplert.New(node.Key(), 1)

	// fails because rt is not set
	invalidOpts := []Option[key.Key256]{}
	req := sm.NewSimRequest[key.Key256](key.ZeroKey256())
	_, err := NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because rt is nil
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](nil),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because endpoint is not set
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because endpoint is nil
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
		WithEndpoint[key.Key256](nil),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because scheduler is not set
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
		WithEndpoint[key.Key256](fendpoint),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because scheduler is nil
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
		WithEndpoint[key.Key256](fendpoint),
		WithScheduler[key.Key256](nil),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because HandleResultsFunc is nil
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
		WithEndpoint[key.Key256](fendpoint),
		WithScheduler[key.Key256](sched),
		WithHandleResultsFunc[key.Key256](nil),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because WithNotifyFailureFunc is nil
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
		WithEndpoint[key.Key256](fendpoint),
		WithScheduler[key.Key256](sched),
		WithNotifyFailureFunc[key.Key256](nil),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because NumberUsefulCloserPeers is 0
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
		WithEndpoint[key.Key256](fendpoint),
		WithScheduler[key.Key256](sched),
		WithNumberUsefulCloserPeers[key.Key256](0),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)

	// fails because Concurrency is 0
	invalidOpts = []Option[key.Key256]{
		WithRoutingTable[key.Key256](rt),
		WithEndpoint[key.Key256](fendpoint),
		WithScheduler[key.Key256](sched),
		WithConcurrency[key.Key256](0),
	}
	_, err = NewSimpleQuery[key.Key256](ctx, req, invalidOpts...)
	require.Error(t, err)
}

func simulationSetup(t *testing.T, ctx context.Context, n, bucketSize int,
	clk clock.Clock, protoID address.ProtocolID, peerstoreTTL time.Duration,
	defaultQueryOpts []Option[key.Key8]) (
	[]address.NodeAddr[key.Key8], []scheduler.AwareScheduler, []endpoint.SimEndpoint[key.Key8],
	[]routing.Table[key.Key8], []server.Server[key.Key8], [][]Option[key.Key8],
) {
	router := fe.NewFakeRouter[key.Key8]()

	ids := make([]address.NodeAddr[key.Key8], n)
	scheds := make([]scheduler.AwareScheduler, n)
	fendpoints := make([]endpoint.SimEndpoint[key.Key8], n)
	rts := make([]routing.Table[key.Key8], n)
	servers := make([]server.Server[key.Key8], n)

	spacing := 256 / n

	for i := 0; i < n; i++ {
		scheds[i] = ss.NewSimpleScheduler(clk)
		ids[i] = kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(uint8(i*spacing))), nil)
		fendpoints[i] = fe.NewFakeEndpoint(ids[i].NodeID(), scheds[i], router)
		rts[i] = simplert.New(ids[i].NodeID().Key(), bucketSize)
		cfg := simserver.DefaultConfig()
		cfg.NumberUsefulCloserPeers = bucketSize
		servers[i] = simserver.NewSimServer[key.Key8](rts[i], fendpoints[i], cfg)
		fendpoints[i].AddRequestHandler(protoID, &sm.SimMessage[key.Key8]{}, servers[i].HandleRequest)
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
	queryOpts := make([][]Option[key.Key8], n)
	for i := 0; i < n; i++ {
		queryOpts[i] = append(defaultQueryOpts,
			WithRoutingTable[key.Key8](rts[i]),
			WithEndpoint[key.Key8](fendpoints[i]),
			WithScheduler[key.Key8](scheds[i]),
		)
	}

	return ids, scheds, fendpoints, rts, servers, queryOpts
}

func getHandleResults[K kad.Key[K]](t *testing.T, req message.MinKadRequestMessage[K],
	expectedPeers []K, expectedResponses [][]K) func(
	ctx context.Context, id address.NodeID[K], resp message.MinKadResponseMessage[K]) (
	bool, []address.NodeID[K]) {
	var responseCount int
	return func(ctx context.Context, id address.NodeID[K],
		resp message.MinKadResponseMessage[K],
	) (bool, []address.NodeID[K]) {
		// check that the request was sent to the correct peer
		require.Equal(t, expectedPeers[responseCount], id.Key(), "responseCount: ", responseCount)

		ids := make([]address.NodeID[K], len(resp.CloserNodes()))
		var found bool
		for i, n := range resp.CloserNodes() {
			ids[i] = n.NodeID()
			if key.Equal(ids[i].Key(), req.Target()) {
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
	defaultQueryOpts := []Option[key.Key8]{
		WithProtocolID[key.Key8](protoID),
		WithConcurrency[key.Key8](1),
		WithNumberUsefulCloserPeers[key.Key8](bucketSize),
		WithRequestTimeout[key.Key8](time.Second),
		WithPeerstoreTTL[key.Key8](peerstoreTTL),
	}

	ids, scheds, _, rts, _, queryOpts := simulationSetup(t, ctx, nPeers,
		bucketSize, clk, protoID, peerstoreTTL, defaultQueryOpts)

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sm.NewSimRequest(ids[len(ids)-1].NodeID().Key())

	// peers that are expected to be queried, in order
	expectedPeers := []key.Key8{}
	// peer that are expected to be included in responses, in order
	expectedResponses := [][]key.Key8{}

	currID := 0
	// while currID != target.Key()
	for !key.Equal(ids[currID].NodeID().Key(), req.Target()) {
		// get closest peer to target from the sollicited peer
		closest, err := rts[currID].NearestPeers(ctx, req.Target(), 1)
		require.NoError(t, err)
		require.Len(t, closest, 1, fmt.Sprint(key.Equal(ids[currID].NodeID().Key(), req.Target())))
		expectedPeers = append(expectedPeers, closest[0].Key())

		// the next current id is the closest peer to the target
		currID = int(closest[0].Key() / 8)

		// the peers included in the response are the closest to the target
		// from the sollicited peer
		responseClosest, err := rts[currID].NearestPeers(ctx, req.Target(), bucketSize)
		require.NoError(t, err)
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
	handleResults := getHandleResults[key.Key8](t, req, expectedPeers, expectedResponses)

	// the request will not fail
	notifyFailure := func(context.Context) {
		require.Fail(t, "notify failure shouldn't be called")
	}

	_, err := NewSimpleQuery[key.Key8](ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc[key.Key8](notifyFailure))...)
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
	defaultQueryOpts := []Option[key.Key8]{
		WithProtocolID[key.Key8](protoID),
		WithConcurrency[key.Key8](1),
		WithNumberUsefulCloserPeers[key.Key8](bucketSize),
		WithRequestTimeout[key.Key8](time.Second),
		WithPeerstoreTTL[key.Key8](peerstoreTTL),
	}

	ids, scheds, _, rts, _, queryOpts := simulationSetup(t, ctx, nPeers,
		bucketSize, clk, protoID, peerstoreTTL, defaultQueryOpts)

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sm.NewSimRequest(key.Key8(0xff))

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
		expectedPeers[i] = ids[o].NodeID().Key()
	}

	// peer that are expected to be included in responses, in order
	expectedResponses := make([][]key.Key8, len(order))
	for i, o := range order {
		// the peers included in the response are the closest to the target
		// from the sollicited peer
		responseClosest, err := rts[o].NearestPeers(ctx, req.Target(), bucketSize)
		require.NoError(t, err)
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
	handleResults := getHandleResults[key.Key8](t, req, expectedPeers, expectedResponses)

	var failed bool
	// the request will not fail
	notifyFailure := func(context.Context) {
		failed = true
	}

	_, err := NewSimpleQuery[key.Key8](ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc[key.Key8](notifyFailure))...)
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

	router := fe.NewFakeRouter[key.Key256]()

	ids := make([]address.NodeAddr[key.Key256], nPeers)
	scheds := make([]scheduler.AwareScheduler, nPeers)
	fendpoints := make([]endpoint.SimEndpoint[key.Key256], nPeers)
	rts := make([]routing.Table[key.Key256], nPeers)
	servers := make([]server.Server[key.Key256], nPeers)

	for i := 0; i < nPeers; i++ {
		scheds[i] = ss.NewSimpleScheduler(clk)
		ids[i] = kadaddr.NewKadAddr(kadid.NewKadID(testutil.Key256WithLeadingBytes([]byte{byte(i * 32)})), nil)
		fendpoints[i] = fe.NewFakeEndpoint(ids[i].NodeID(), scheds[i], router)
		rts[i] = simplert.New(ids[i].NodeID().Key(), bucketSize)
		servers[i] = basicserver.NewBasicServer(rts[i], fendpoints[i],
			basicserver.WithNumberUsefulCloserPeers(bucketSize))
		fendpoints[i].AddRequestHandler(protoID, &sm.SimMessage[key.Key8]{}, servers[i].HandleRequest)
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
			_, err = rts[c[i]].AddPeer(ctx, ids[c[1-i]].NodeID())
			require.NoError(t, err)
		}
	}

	// generic query options to be used by all peers
	defaultQueryOpts := []Option[key.Key256]{
		WithProtocolID[key.Key256](protoID),
		WithConcurrency[key.Key256](2),
		WithNumberUsefulCloserPeers[key.Key256](bucketSize),
		WithRequestTimeout[key.Key256](time.Second),
		WithPeerstoreTTL[key.Key256](peerstoreTTL),
	}

	// query options for each peer
	queryOpts := make([][]Option[key.Key256], nPeers)
	for i := 0; i < nPeers; i++ {
		queryOpts[i] = append(defaultQueryOpts,
			WithRoutingTable(rts[i]),
			WithEndpoint[key.Key256](fendpoints[i]),
			WithScheduler[key.Key256](scheds[i]),
		)
	}

	// smallest peer is looking for biggest peer (which is the most far away
	// in hop numbers, given the routing table configuration)
	req := sm.NewSimRequest(ids[len(ids)-1].NodeID().Key())

	// peers that are expected to be queried, in order
	expectedPeers := []key.Key256{
		ids[3].NodeID().Key(), ids[2].NodeID().Key(),
		ids[4].NodeID().Key(), ids[6].NodeID().Key(),
	}
	// peer that are expected to be included in responses, in order
	expectedResponses := [][]key.Key256{
		{ids[4].NodeID().Key(), ids[0].NodeID().Key()},
		{ids[6].NodeID().Key(), ids[0].NodeID().Key()},
		{ids[5].NodeID().Key(), ids[3].NodeID().Key()},
		{ids[7].NodeID().Key(), ids[2].NodeID().Key(), ids[5].NodeID().Key()},
	}

	handleResults := getHandleResults[key.Key256](t, req, expectedPeers, expectedResponses)

	// the request will not fail
	notifyFailure := func(context.Context) {
		require.Fail(t, "notify failure shouldn't be called")
	}

	_, err := NewSimpleQuery[key.Key256](ctx, req, append(queryOpts[0],
		WithHandleResultsFunc(handleResults),
		WithNotifyFailureFunc[key.Key256](notifyFailure))...)
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

	router := fe.NewFakeRouter[key.Key8]()
	node0 := kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0)), nil)
	node1 := kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(1)), nil)
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint[key.Key8](node0.NodeID(), sched0, router)
	fendpoint1 := fe.NewFakeEndpoint[key.Key8](node1.NodeID(), sched1, router)
	rt0 := simplert.New(node0.NodeID().Key(), bucketSize)

	serverRequestHandler := func(context.Context, address.NodeID[key.Key8],
		message.MinKadMessage,
	) (message.MinKadMessage, error) {
		return nil, errors.New("")
	}
	fendpoint1.AddRequestHandler(protoID, &sm.SimMessage[key.Key8]{}, serverRequestHandler)

	req := sm.NewSimRequest(key.Key8(0xff))

	responseHandler := func(ctx context.Context, sender address.NodeID[key.Key8],
		msg message.MinKadResponseMessage[key.Key8],
	) (bool, []address.NodeID[key.Key8]) {
		require.Fail(t, "response handler shouldn't be called")
		return false, nil
	}
	queryOpts := []Option[key.Key8]{
		WithProtocolID[key.Key8](protoID),
		WithConcurrency[key.Key8](2),
		WithNumberUsefulCloserPeers[key.Key8](bucketSize),
		WithRequestTimeout[key.Key8](time.Millisecond),
		WithEndpoint[key.Key8](fendpoint0),
		WithRoutingTable[key.Key8](rt0),
		WithScheduler[key.Key8](sched0),
		WithHandleResultsFunc(responseHandler),
	}

	// query creation fails because the routing table of 0 is empty
	_, err := NewSimpleQuery[key.Key8](ctx, req, queryOpts...)
	require.Error(t, err)

	// connect 0 and 1
	err = fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)
	require.NoError(t, err)
	success, err := rt0.AddPeer(ctx, node1.NodeID())
	require.NoError(t, err)
	require.True(t, success)

	q, err := NewSimpleQuery[key.Key8](ctx, req, queryOpts...)
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

	router := fe.NewFakeRouter[key.Key8]()
	node0 := kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0x00)), nil)
	sched0 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint(node0.NodeID(), sched0, router)
	rt0 := simplert.New(node0.NodeID().Key(), bucketSize)

	node1 := kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0x01)), nil)
	fendpoint0.MaybeAddToPeerstore(ctx, node1, peerstoreTTL)

	success, err := rt0.AddPeer(ctx, node1.NodeID())
	require.NoError(t, err)
	require.True(t, success)

	req := sm.NewSimRequest(key.Key8(0xff))

	responseHandler := func(ctx context.Context, sender address.NodeID[key.Key8],
		msg message.MinKadResponseMessage[key.Key8],
	) (bool, []address.NodeID[key.Key8]) {
		ids := make([]address.NodeID[key.Key8], len(msg.CloserNodes()))
		for i, peer := range msg.CloserNodes() {
			ids[i] = peer.NodeID()
		}
		return false, ids
	}

	queryOpts := []Option[key.Key8]{
		WithProtocolID[key.Key8](protoID),
		WithConcurrency[key.Key8](1),
		WithNumberUsefulCloserPeers[key.Key8](bucketSize),
		WithRequestTimeout[key.Key8](time.Millisecond),
		WithEndpoint[key.Key8](fendpoint0),
		WithRoutingTable[key.Key8](rt0),
		WithScheduler[key.Key8](sched0),
		WithHandleResultsFunc(responseHandler),
	}

	q, err := NewSimpleQuery[key.Key8](ctx, req, queryOpts...)
	require.NoError(t, err)

	// nil response should trigger a request error
	q.handleResponse(ctx, node1.NodeID(), nil)

	addrs := []address.NodeAddr[key.Key8]{
		kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0xee)), nil),
		// kadaddr.NewKadAddr(kadid.NewKadID([]byte{0x66, 0x66}), nil), // invalid key length
		kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0x88)), nil),
	}
	forgedResponse := sm.NewSimResponse(addrs)
	q.handleResponse(ctx, node1.NodeID(), forgedResponse)

	// test that 0xee and 0x88 have been added to peerlist but not 0x6666
	require.Equal(t, addrs[0].NodeID(), q.peerlist.closest.id)
	require.Equal(t, addrs[1].NodeID(), q.peerlist.closest.next.id)
	require.Equal(t, node1.NodeID(), q.peerlist.closest.next.next.id)
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
		q.requestError(ctx, node1.NodeID(), errors.New(""))
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

	queryOpts := []Option[key.Key256]{
		WithProtocolID[key.Key256](protoID),
		WithConcurrency[key.Key256](1),
		WithNumberUsefulCloserPeers[key.Key256](bucketSize),
		WithRequestTimeout[key.Key256](time.Millisecond),
		WithEndpoint[key.Key256](libp2pEndpoint),
		WithRoutingTable[key.Key256](rt),
		WithScheduler[key.Key256](sched),
	}

	req := sm.NewSimRequest(si.NewStringID("RandomKey").Key())

	q2, err := NewSimpleQuery[key.Key256](ctx, req, queryOpts...)
	require.NoError(t, err)

	// set the peerlist endpoint to nil, to allow invalid NodeIDs in peerlist
	q2.peerlist.endpoint = nil
	// change the node id of the queued peer. sending the message with
	// SendRequestHandleResponse will fail fast (no new go routine created)
	q2.peerlist.closest.id = kadid.NewKadID(key.ZeroKey256())

	require.True(t, sched.RunOne(ctx))
	require.False(t, sched.RunOne(ctx))
}
