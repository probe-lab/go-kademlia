package main

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/endpoint/fakeendpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/simmessage"
	sq "github.com/plprobelab/go-kademlia/query/simplequery"
	"github.com/plprobelab/go-kademlia/routingtable"
	"github.com/plprobelab/go-kademlia/routingtable/simplert"
	"github.com/plprobelab/go-kademlia/server"
	"github.com/plprobelab/go-kademlia/server/basicserver"
	"github.com/plprobelab/go-kademlia/util"

	"github.com/plprobelab/go-kademlia/events/scheduler"
	ss "github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/events/simulator"
	"github.com/plprobelab/go-kademlia/events/simulator/litesimulator"
)

const (
	keysize      = 1                                 // keysize in bytes
	peerstoreTTL = 10 * time.Minute                  // duration for which a peer is kept in the peerstore
	protoID      = address.ProtocolID("/test/1.0.0") // protocol ID for the test
)

// connectNodes adds nodes to each other's peerstores and routing tables
func connectNodes(ctx context.Context, n0, n1 address.NodeID, ep0, ep1 endpoint.Endpoint,
	rt0, rt1 routingtable.RoutingTable) {
	// add n1 to n0's peerstore and routing table
	ep0.MaybeAddToPeerstore(ctx, n1, peerstoreTTL)
	rt0.AddPeer(ctx, n1)
	// add n0 to n1's peerstore and routing table
	ep1.MaybeAddToPeerstore(ctx, n0, peerstoreTTL)
	rt1.AddPeer(ctx, n0)
}

func findNode(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "findNode test")
	defer span.End()

	// create mock clock to control time
	clk := clock.NewMock()
	// create a fake router to virtually connect nodes
	router := fakeendpoint.NewFakeRouter()

	// create node identifiers
	nodeCount := 4
	ids := make([]*kadid.KadID, nodeCount)
	ids[0] = kadid.NewKadID(key.KadKey(make([]byte, keysize)))
	ids[1] = kadid.NewKadID(key.KadKey(append(make([]byte, keysize-1), 0x01)))
	ids[2] = kadid.NewKadID(key.KadKey(append(make([]byte, keysize-1), 0x02)))
	ids[3] = kadid.NewKadID(key.KadKey(append(make([]byte, keysize-1), 0x03)))

	// Kademlia trie:
	//     ^
	//    / \
	//   ^   ^
	//  A B C D

	rts := make([]*simplert.SimpleRT, len(ids))
	eps := make([]*fakeendpoint.FakeEndpoint, len(ids))
	schedulers := make([]scheduler.AwareScheduler, len(ids))
	servers := make([]server.Server, len(ids))

	for i := 0; i < len(ids); i++ {
		// create a routing table, with bucket size 2
		rts[i] = simplert.NewSimpleRT(ids[i].KadKey, 2)
		// create a scheduler based on the mock clock
		schedulers[i] = ss.NewSimpleScheduler(clk)
		// create a fake endpoint for the node, communicating through the router
		eps[i] = fakeendpoint.NewFakeEndpoint(ids[i], schedulers[i], router)
		// create a server instance for the node
		servers[i] = basicserver.NewBasicServer(rts[i], eps[i])
		// add the server request handler for protoID to the endpoint
		eps[i].AddRequestHandler(protoID, servers[i].HandleRequest, nil)
	}

	// A connects to B
	connectNodes(ctx, ids[0], ids[1], eps[0], eps[1], rts[0], rts[1])

	// B connects to C
	connectNodes(ctx, ids[1], ids[2], eps[1], eps[2], rts[1], rts[2])

	// C connects to D
	connectNodes(ctx, ids[2], ids[3], eps[2], eps[3], rts[2], rts[3])

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)
	req := simmessage.NewSimRequest(ids[3].Key())

	// handleResFn is called when a response is received during the query process
	handleResFn := func(_ context.Context, id address.NodeID,
		msg message.MinKadResponseMessage) (bool, []address.NodeID) {
		resp := msg.(*simmessage.SimMessage)
		fmt.Println("got a response from", id, "with", resp.CloserNodes())

		for _, peer := range resp.CloserNodes() {
			if peer.String() == ids[3].NodeID().String() {
				// the response contains the address of D (ids[3])
				fmt.Println("success")
				// returning true will stop the query process
				return true, nil
			}
		}
		// returning false will continue the query process
		return false, resp.CloserNodes()
	}

	// create a query on A (using A's scheduler, endpoint and routing table),
	// D's Kademlia Key as target, the defined protocol ID, using req as the
	// request message, an empty SimMessage (resp) as the response message, a
	// concurrency of 1, a timeout of 1 second, and handleResFn as the response
	// handler. The query doesn't run yet, it is added to A's event queue
	// through A's scheduler.
	queryOpts := []sq.Option{
		sq.WithProtocolID(protoID),
		sq.WithConcurrency(1),
		sq.WithRequestTimeout(time.Second),
		sq.WithHandleResultsFunc(handleResFn),
		sq.WithRoutingTable(rts[0]),
		sq.WithEndpoint(eps[0]),
		sq.WithScheduler(schedulers[0]),
	}
	sq.NewSimpleQuery(ctx, req, queryOpts...)

	// create a simulator, simulating [A, B, C, D]'s simulators
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, schedulers...)

	// run the simulation until all events are processed
	sim.Run(ctx)
}
