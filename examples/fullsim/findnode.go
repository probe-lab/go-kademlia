package main

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadaddr"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	sq "github.com/plprobelab/go-kademlia/query/simplequery"
	"github.com/plprobelab/go-kademlia/routing"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/server"
	"github.com/plprobelab/go-kademlia/sim"
	"github.com/plprobelab/go-kademlia/util"

	"github.com/plprobelab/go-kademlia/events/scheduler"
	ss "github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/events/simulator"
	"github.com/plprobelab/go-kademlia/events/simulator/litesimulator"
)

const (
	peerstoreTTL = 10 * time.Minute                  // duration for which a peer is kept in the peerstore
	protoID      = address.ProtocolID("/test/1.0.0") // protocol ID for the test
)

// connectNodes adds nodes to each other's peerstores and routing tables
func connectNodes(ctx context.Context, n0, n1 address.NodeAddr[key.Key8], ep0, ep1 endpoint.Endpoint[key.Key8],
	rt0, rt1 routing.Table[key.Key8],
) {
	// add n1 to n0's peerstore and routing table
	ep0.MaybeAddToPeerstore(ctx, n1, peerstoreTTL)
	rt0.AddPeer(ctx, n1.NodeID())
	// add n0 to n1's peerstore and routing table
	ep1.MaybeAddToPeerstore(ctx, n0, peerstoreTTL)
	rt1.AddPeer(ctx, n0.NodeID())
}

func findNode(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "findNode test")
	defer span.End()

	// create mock clock to control time
	clk := clock.NewMock()
	// create a fake router to virtually connect nodes
	router := sim.NewRouter[key.Key8]()

	// create node identifiers
	nodeCount := 4
	nodes := make([]*kadaddr.KadAddr[key.Key8], nodeCount)
	nodes[0] = kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0)), nil)
	nodes[1] = kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0x01)), nil)
	nodes[2] = kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0x02)), nil)
	nodes[3] = kadaddr.NewKadAddr(kadid.NewKadID(key.Key8(0x03)), nil)

	// Kademlia trie:
	//     ^
	//    / \
	//   ^   ^
	//  A B C D

	rts := make([]*simplert.SimpleRT[key.Key8], len(nodes))
	eps := make([]*sim.Endpoint[key.Key8], len(nodes))
	schedulers := make([]scheduler.AwareScheduler, len(nodes))
	servers := make([]server.Server[key.Key8], len(nodes))

	for i := 0; i < len(nodes); i++ {
		// create a routing table, with bucket size 2
		rts[i] = simplert.New(nodes[i].NodeID().Key(), 2)
		// create a scheduler based on the mock clock
		schedulers[i] = ss.NewSimpleScheduler(clk)
		// create a fake endpoint for the node, communicating through the router
		eps[i] = sim.NewEndpoint(nodes[i].NodeID(), schedulers[i], router)
		// create a server instance for the node
		servers[i] = sim.NewServer[key.Key8](rts[i], eps[i], sim.DefaultConfig())
		// add the server request handler for protoID to the endpoint
		err := eps[i].AddRequestHandler(protoID, nil, servers[i].HandleRequest)
		if err != nil {
			panic(err)
		}
	}

	// A connects to B
	connectNodes(ctx, nodes[0], nodes[1], eps[0], eps[1], rts[0], rts[1])

	// B connects to C
	connectNodes(ctx, nodes[1], nodes[2], eps[1], eps[2], rts[1], rts[2])

	// C connects to D
	connectNodes(ctx, nodes[2], nodes[3], eps[2], eps[3], rts[2], rts[3])

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)
	req := sim.NewRequest(nodes[3].NodeID().Key())

	// handleResFn is called when a response is received during the query process
	handleResFn := func(_ context.Context, id address.NodeID[key.Key8],
		msg message.MinKadResponseMessage[key.Key8],
	) (bool, []address.NodeID[key.Key8]) {
		resp := msg.(*sim.Message[key.Key8])
		fmt.Println("got a response from", id, "with", resp.CloserNodes())

		newIds := make([]address.NodeID[key.Key8], len(resp.CloserNodes()))
		for i, peer := range resp.CloserNodes() {
			if peer.NodeID().String() == nodes[3].NodeID().String() {
				// the response contains the address of D (ids[3])
				fmt.Println("success")
				// returning true will stop the query process
				return true, nil
			}
			newIds[i] = peer.NodeID()
		}
		// returning false will continue the query process
		return false, newIds
	}

	// create a query on A (using A's scheduler, endpoint and routing table),
	// D's Kademlia Key as target, the defined protocol ID, using req as the
	// request message, an empty SimMessage (resp) as the response message, a
	// concurrency of 1, a timeout of 1 second, and handleResFn as the response
	// handler. The query doesn't run yet, it is added to A's event queue
	// through A's scheduler.
	queryOpts := []sq.Option[key.Key8]{
		sq.WithProtocolID[key.Key8](protoID),
		sq.WithConcurrency[key.Key8](1),
		sq.WithRequestTimeout[key.Key8](time.Second),
		sq.WithHandleResultsFunc(handleResFn),
		sq.WithRoutingTable[key.Key8](rts[0]),
		sq.WithEndpoint[key.Key8](eps[0]),
		sq.WithScheduler[key.Key8](schedulers[0]),
	}
	sq.NewSimpleQuery[key.Key8](ctx, req, queryOpts...)

	// create a simulator, simulating [A, B, C, D]'s simulators
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, schedulers...)

	// run the simulation until all events are processed
	sim.Run(ctx)
}
