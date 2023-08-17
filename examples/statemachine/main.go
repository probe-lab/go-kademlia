package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/benbjohnson/clock"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"github.com/plprobelab/go-kademlia/coord"
	"github.com/plprobelab/go-kademlia/event"
	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/sim"
	"github.com/plprobelab/go-kademlia/util"
)

func main() {
	ctx := context.Background()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	nodes, eps, rts, scheds, siml := setupSimulation(ctx)

	tp, err := tracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		log.Fatal(err)
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)

	ctx, span := util.StartSpan(ctx, "main")
	defer span.End()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}(ctx)

	ccfg := coord.DefaultConfig()
	ccfg.Clock = siml.Clock()
	ccfg.PeerstoreTTL = peerstoreTTL

	kad, err := coord.NewCoordinator[key.Key256, net.IP](nodes[0].ID(), eps[0], findNodeFn, rts[0], scheds[0], ccfg)
	if err != nil {
		log.Fatal(err)
	}

	ih := NewIpfsDht(kad)
	ih.Start(ctx)

	go func(ctx context.Context) {
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				debug("Running simulator")
				siml.Run(ctx)
			case <-ctx.Done():
				debug("Exiting simulator")
				return
			}
		}
	}(ctx)

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)

	addr, err := ih.FindNode(ctx, nodes[3].ID())
	if err != nil {
		fmt.Printf("FindNode failed with error: %v\n", err)
		return
	}
	fmt.Printf("FindNode found address for: %s\n", addr.ID().String())
}

const peerstoreTTL = 10 * time.Minute

func setupSimulation(ctx context.Context) ([]kad.NodeInfo[key.Key256, net.IP], []*sim.Endpoint[key.Key256, net.IP], []kad.RoutingTable[key.Key256, kad.NodeID[key.Key256]], []event.AwareScheduler, *sim.LiteSimulator) {
	// create node identifiers
	nodeCount := 4
	ids := make([]*kadtest.ID[key.Key256], nodeCount)
	ids[0] = kadtest.NewID(key.ZeroKey256())
	ids[1] = kadtest.NewID(key.NewKey256(append(make([]byte, 31), 0x01)))
	ids[2] = kadtest.NewID(key.NewKey256(append(make([]byte, 31), 0x02)))
	ids[3] = kadtest.NewID(key.NewKey256(append(make([]byte, 31), 0x03)))

	// Kademlia trie:
	//     ^
	//    / \
	//   ^   ^
	//  A B C D

	addrs := make([]kad.NodeInfo[key.Key256, net.IP], nodeCount)
	for i := 0; i < nodeCount; i++ {
		addrs[i] = kadtest.NewInfo(ids[i], []net.IP{})
	}

	// makeEndpointRequestHandler returns an Endpoint RequestHandler that adapts the custom protocol messages
	// into sim messages
	makeEndpointRequestHandler := func(s *sim.Server[key.Key256, net.IP]) endpoint.RequestHandlerFn[key.Key256] {
		return func(ctx context.Context, rpeer kad.NodeID[key.Key256], msg kad.Message) (kad.Message, error) {
			// Adapt the endpoint to use the statemachine example protocol messages
			switch msg := msg.(type) {
			case *FindNodeRequest[key.Key256, net.IP]:
				resp, err := s.HandleFindNodeRequest(ctx, rpeer, sim.NewRequest[key.Key256, net.IP](msg.Target()))
				if err != nil {
					return nil, err
				}
				switch resp := resp.(type) {
				case kad.Response[key.Key256, net.IP]:
					return &FindNodeResponse[key.Key256, net.IP]{
						CloserPeers: resp.CloserNodes(),
					}, nil
				default:
					return nil, sim.ErrUnknownMessageFormat
				}

			default:
				return nil, sim.ErrUnknownMessageFormat
			}
		}
	}
	// create mock clock to control time
	clk := clock.NewMock()

	// create a fake router to virtually connect nodes
	router := sim.NewRouter[key.Key256, net.IP]()

	rts := make([]kad.RoutingTable[key.Key256, kad.NodeID[key.Key256]], len(addrs))
	eps := make([]*sim.Endpoint[key.Key256, net.IP], len(addrs))
	schedulers := make([]event.AwareScheduler, len(addrs))
	servers := make([]*sim.Server[key.Key256, net.IP], len(addrs))

	for i := 0; i < len(addrs); i++ {
		i := i // :(
		// create a routing table, with bucket size 2
		rts[i] = simplert.New[key.Key256, kad.NodeID[key.Key256]](addrs[i].ID(), 2)
		// create a scheduler based on the mock clock
		schedulers[i] = event.NewSimpleScheduler(clk)
		// create a fake endpoint for the node, communicating through the router
		eps[i] = sim.NewEndpoint[key.Key256, net.IP](addrs[i].ID(), schedulers[i], router)
		// create a server instance for the node
		servers[i] = sim.NewServer[key.Key256, net.IP](rts[i], eps[i], sim.DefaultServerConfig())
		// add the server request handler for protoID to the endpoint
		err := eps[i].AddRequestHandler(protoID, nil, makeEndpointRequestHandler(servers[i]))
		if err != nil {
			panic(err)
		}
	}

	// A connects to B
	connectNodes(ctx, addrs[0], addrs[1], eps[0], eps[1], rts[0], rts[1])

	// B connects to C
	connectNodes(ctx, addrs[1], addrs[2], eps[1], eps[2], rts[1], rts[2])

	// C connects to D
	connectNodes(ctx, addrs[2], addrs[3], eps[2], eps[3], rts[2], rts[3])

	// create a simulator, simulating [A, B, C, D]'s simulators
	siml := sim.NewLiteSimulator(clk)
	sim.AddSchedulers(siml, schedulers...)

	return addrs, eps, rts, schedulers, siml
}

// connectNodes adds nodes to each other's peerstores and routing tables
func connectNodes(ctx context.Context, n0, n1 kad.NodeInfo[key.Key256, net.IP], ep0, ep1 endpoint.Endpoint[key.Key256, net.IP],
	rt0, rt1 kad.RoutingTable[key.Key256, kad.NodeID[key.Key256]],
) {
	// add n1 to n0's peerstore and routing table
	debug("connecting %s to %s", n0.ID(), n1.ID())
	ep0.MaybeAddToPeerstore(ctx, n1, peerstoreTTL)
	rt0.AddNode(n1.ID())

	// add n0 to n1's peerstore and routing table
	debug("connecting %s to %s", n1.ID(), n0.ID())
	ep1.MaybeAddToPeerstore(ctx, n0, peerstoreTTL)
	rt1.AddNode(n0.ID())
}

func debug(f string, args ...any) {
	fmt.Println(fmt.Sprintf(f, args...))
}

var findNodeFn = func(n kad.NodeID[key.Key256]) (address.ProtocolID, kad.Request[key.Key256, net.IP]) {
	return protoID, sim.NewRequest[key.Key256, net.IP](n.Key())
}

type RoutingUpdate any

type Event any

// tracerProvider returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func tracerProvider(url string) (*trace.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := trace.NewTracerProvider(
		// Always be sure to batch in production.
		trace.WithBatcher(exp),
		// Record information about this application in a Resource.
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("Kademlia-Test"),
			semconv.ServiceVersion("v0.1.0"),
			attribute.String("environment", "demo"),
		)),
	)
	return tp, nil
}
