package main

import (
	"context"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/plprobelab/go-kademlia/routing/triert"

	"github.com/libp2p/go-libp2p"
	kadp2p "github.com/plprobelab/go-kademlia/libp2p"

	ma "github.com/multiformats/go-multiaddr"
	//"github.com/plprobelab/go-kademlia/libp2p"

	"github.com/plprobelab/go-kademlia/coord"
	"github.com/plprobelab/go-kademlia/key"
)

func main() {
	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ccfg := coord.DefaultConfig()

	h, err := libp2p.New()
	if err != nil {
		panic(err)
	}

	// friend is the first peer we know in the IPFS DHT network (bootstrap node)
	friend, err := peer.Decode("12D3KooWGjgvfDkpuVAoNhd7PRRvMTEG4ZgzHBFURqDe1mqEzAMS")
	if err != nil {
		panic(err)
	}
	friendID := kadp2p.NewPeerID(friend)

	a, err := ma.NewMultiaddr("/ip4/45.32.75.236/udp/4001/quic")
	if err != nil {
		panic(err)
	}
	// connect to friend
	friendAddr := peer.AddrInfo{ID: friend, Addrs: []ma.Multiaddr{a}}
	if err := h.Connect(ctx, friendAddr); err != nil {
		panic(err)
	}
	fmt.Println("connected to friend")

	f := &kadp2p.ProtocolFindNode{
		Host: h,
	}

	// target is the peer we want to find QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb
	target, err := peer.Decode("QmSKVUFAyCddg2wDUdZVCfvqG5YCwwJTWY1HRmorebXcKG")
	if err != nil {
		panic(err)
	}
	targetID := kadp2p.NewPeerID(target)

	rt, err := triert.New(kadp2p.NewPeerID(h.ID()).Key(), triert.DefaultConfig[key.Key256]())
	if err != nil {
		panic(err)
	}

	success := rt.AddNode(friendID)
	if !success {
		panic("failed to add friend to rt")
	}

	fmt.Println("new coordinator", h.ID())
	kad, err := coord.NewCoordinator[key.Key256, kadp2p.PeerID, ma.Multiaddr](kadp2p.NewPeerID(h.ID()), f, rt, ccfg)
	if err != nil {
		log.Fatal(err)
	}
	kad.Start(ctx)
	//
	//go func(ctx context.Context) {
	//	for {
	//		select {
	//		case <-time.After(100 * time.Millisecond):
	//			debug("Running simulator")
	//			siml.Run(ctx)
	//		case <-ctx.Done():
	//			debug("Exiting simulator")
	//			return
	//		}
	//	}
	//}(ctx)

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)

	//addr, err := ih.FindNode(ctx, nodes[3].ID())
	//if err != nil {
	//	fmt.Printf("FindNode failed with error: %v\n", err)
	//	return
	//}
	//fmt.Printf("FindNode found address for: %s\n", addr.ID().String())

	fmt.Println("FindNode", targetID, targetID.Key().HexString())
	node, err := kad.FindNode(ctx, targetID)
	if err != nil {
		fmt.Printf("FindNode failed with error: %v\n", err)
		return
	}
	fmt.Printf("FindNode found address for: %s\n", node.ID())
	for _, a := range node.Addresses() {
		fmt.Println("  -", a.String())
	}
}

//const peerstoreTTL = 10 * time.Minute
//
//func setupSimulation(ctx context.Context) ([]kad.NodeInfo[key.Key256, net.IP], []*sim.Endpoint[key.Key256, net.IP], []kad.RoutingTable[key.Key256], *litesimulator.LiteSimulator) {
//	// create node identifiers
//	nodeCount := 4
//	ids := make([]*kadtest.ID[key.Key256], nodeCount)
//	ids[0] = kadtest.NewID(key.ZeroKey256())
//	ids[1] = kadtest.NewID(key.NewKey256(append(make([]byte, 31), 0x01)))
//	ids[2] = kadtest.NewID(key.NewKey256(append(make([]byte, 31), 0x02)))
//	ids[3] = kadtest.NewID(key.NewKey256(append(make([]byte, 31), 0x03)))
//
//	// Kademlia trie:
//	//     ^
//	//    / \
//	//   ^   ^
//	//  A B C D
//
//	addrs := make([]kad.NodeInfo[key.Key256, net.IP], nodeCount)
//	for i := 0; i < nodeCount; i++ {
//		addrs[i] = kadtest.NewInfo(ids[i], []net.IP{})
//	}
//
//	// makeEndpointRequestHandler returns an Endpoint RequestHandler that adapts the custom protocol messages
//	// into sim messages
//	makeEndpointRequestHandler := func(s *sim.Server[key.Key256, net.IP]) endpoint.RequestHandlerFn[key.Key256] {
//		return func(ctx context.Context, rpeer kad.NodeID[key.Key256], msg kad.Message) (kad.Message, error) {
//			// Adapt the endpoint to use the statemachine example protocol messages
//			switch msg := msg.(type) {
//			case *FindNodeRequest[key.Key256, net.IP]:
//				resp, err := s.HandleFindNodeRequest(ctx, rpeer, sim.NewRequest[key.Key256, net.IP](msg.Target()))
//				if err != nil {
//					return nil, err
//				}
//				switch resp := resp.(type) {
//				case kad.Response[key.Key256, net.IP]:
//					return &FindNodeResponse[key.Key256, net.IP]{
//						CloserPeers: resp.CloserNodes(),
//					}, nil
//				default:
//					return nil, sim.ErrUnknownMessageFormat
//				}
//
//			default:
//				return nil, sim.ErrUnknownMessageFormat
//			}
//		}
//	}
//	// create mock clock to control time
//	clk := clock.NewMock()
//
//	// create a fake router to virtually connect nodes
//	router := sim.NewRouter[key.Key256, net.IP]()
//
//	rts := make([]kad.RoutingTable[key.Key256], len(addrs))
//	eps := make([]*sim.Endpoint[key.Key256, net.IP], len(addrs))
//	schedulers := make([]scheduler.AwareScheduler, len(addrs))
//	servers := make([]*sim.Server[key.Key256, net.IP], len(addrs))
//
//	for i := 0; i < len(addrs); i++ {
//		i := i // :(
//		// create a routing table, with bucket size 2
//		rts[i] = simplert.New(addrs[i].ID().Key(), 2)
//		// create a scheduler based on the mock clock
//		schedulers[i] = ss.NewSimpleScheduler(clk)
//		// create a fake endpoint for the node, communicating through the router
//		eps[i] = sim.NewEndpoint[key.Key256, net.IP](addrs[i].ID(), schedulers[i], router)
//		// create a server instance for the node
//		servers[i] = sim.NewServer[key.Key256, net.IP](rts[i], eps[i], sim.DefaultServerConfig())
//		// add the server request handler for protoID to the endpoint
//		err := eps[i].AddRequestHandler(protoID, nil, makeEndpointRequestHandler(servers[i]))
//		if err != nil {
//			panic(err)
//		}
//	}
//
//	// A connects to B
//	connectNodes(ctx, addrs[0], addrs[1], eps[0], eps[1], rts[0], rts[1])
//
//	// B connects to C
//	connectNodes(ctx, addrs[1], addrs[2], eps[1], eps[2], rts[1], rts[2])
//
//	// C connects to D
//	connectNodes(ctx, addrs[2], addrs[3], eps[2], eps[3], rts[2], rts[3])
//
//	// create a simulator, simulating [A, B, C, D]'s simulators
//	siml := litesimulator.NewLiteSimulator(clk)
//	simulator.AddPeers(siml, schedulers...)
//
//	return addrs, eps, rts, siml
//}
//
//// connectNodes adds nodes to each other's peerstores and routing tables
//func connectNodes(ctx context.Context, n0, n1 kad.NodeInfo[key.Key256, net.IP], ep0, ep1 endpoint.Endpoint[key.Key256, net.IP],
//	rt0, rt1 kad.RoutingTable[key.Key256],
//) {
//	// add n1 to n0's peerstore and routing table
//	debug("connecting %s to %s", n0.ID(), n1.ID())
//	ep0.MaybeAddToPeerstore(ctx, n1, peerstoreTTL)
//	rt0.AddNode(n1.ID())
//
//	// add n0 to n1's peerstore and routing table
//	debug("connecting %s to %s", n1.ID(), n0.ID())
//	ep1.MaybeAddToPeerstore(ctx, n0, peerstoreTTL)
//	rt1.AddNode(n0.ID())
//}
//
//func debug(f string, args ...any) {
//	fmt.Println(fmt.Sprintf(f, args...))
//}
//
//type RoutingUpdate any
//
//type Event any
//
//// tracerProvider returns an OpenTelemetry TracerProvider configured to use
//// the Jaeger exporter that will send spans to the provided url. The returned
//// TracerProvider will also use a Resource configured with all the information
//// about the application.
//func tracerProvider(url string) (*trace.TracerProvider, error) {
//	// Create the Jaeger exporter
//	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
//	if err != nil {
//		return nil, err
//	}
//	tp := trace.NewTracerProvider(
//		// Always be sure to batch in production.
//		trace.WithBatcher(exp),
//		// Record information about this application in a Resource.
//		trace.WithResource(resource.NewWithAttributes(
//			semconv.SchemaURL,
//			semconv.ServiceName("Kademlia-Test"),
//			semconv.ServiceVersion("v0.1.0"),
//			attribute.String("environment", "demo"),
//		)),
//	)
//	return tp, nil
//}
