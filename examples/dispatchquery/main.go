package main

import (
	"context"
	"log"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multibase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"github.com/probe-lab/go-kademlia/event"
	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
	"github.com/probe-lab/go-kademlia/libp2p"
	sq "github.com/probe-lab/go-kademlia/query/simplequery"
	"github.com/probe-lab/go-kademlia/routing/simplert"
	"github.com/probe-lab/go-kademlia/server/basicserver"
	"github.com/probe-lab/go-kademlia/sim"
	"github.com/probe-lab/go-kademlia/util"
)

const (
	peerstoreTTL = 10 * time.Minute
	protoID      = "/ipfs/kad/1.0.0"
)

var targetBytesID = "mACQIARIgp9PBu+JuU8aicuW8xT+Oa08OntMyqdLbfQtOplAHlME"

func queryTest(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "queryTest")
	defer span.End()

	clk := clock.NewMock()

	router := sim.NewRouter[key.Key256, multiaddr.Multiaddr]()

	// create peer A
	pidA, err := peer.Decode("12BooooALPHA")
	if err != nil {
		panic(err)
	}
	selfA := &libp2p.PeerID{ID: pidA} // peer.ID is necessary for ipfskadv1 message format
	addrA := multiaddr.StringCast("/ip4/1.1.1.1/tcp/4001/")
	var naddrA kad.NodeInfo[key.Key256, multiaddr.Multiaddr] = libp2p.NewAddrInfo(peer.AddrInfo{
		ID:    selfA.ID,
		Addrs: []multiaddr.Multiaddr{addrA},
	})
	rtA := simplert.New[key.Key256, kad.NodeID[key.Key256]](selfA, 2)
	schedA := event.NewSimpleScheduler(clk)
	endpointA := sim.NewEndpoint(selfA.NodeID(), schedA, router)
	servA := basicserver.NewBasicServer[multiaddr.Multiaddr](rtA, endpointA)
	err = endpointA.AddRequestHandler(protoID, nil, servA.HandleRequest)
	if err != nil {
		panic(err)
	}

	// create peer B
	pidB, err := peer.Decode("12BoooooBETA")
	if err != nil {
		panic(err)
	}
	selfB := &libp2p.PeerID{ID: pidB}
	addrB := multiaddr.StringCast("/ip4/2.2.2.2/tcp/4001/")
	var naddrB kad.NodeInfo[key.Key256, multiaddr.Multiaddr] = libp2p.NewAddrInfo(peer.AddrInfo{
		ID:    selfB.ID,
		Addrs: []multiaddr.Multiaddr{addrB},
	})
	rtB := simplert.New[key.Key256, kad.NodeID[key.Key256]](selfB, 2)
	schedB := event.NewSimpleScheduler(clk)
	endpointB := sim.NewEndpoint(selfB.NodeID(), schedB, router)
	servB := basicserver.NewBasicServer[multiaddr.Multiaddr](rtB, endpointB)
	err = endpointB.AddRequestHandler(protoID, nil, servB.HandleRequest)
	if err != nil {
		panic(err)
	}

	// create peer C
	pidC, err := peer.Decode("12BooooGAMMA")
	if err != nil {
		panic(err)
	}
	selfC := &libp2p.PeerID{ID: pidC}
	addrC := multiaddr.StringCast("/ip4/3.3.3.3/tcp/4001/")
	var naddrC kad.NodeInfo[key.Key256, multiaddr.Multiaddr] = libp2p.NewAddrInfo(peer.AddrInfo{
		ID:    selfC.ID,
		Addrs: []multiaddr.Multiaddr{addrC},
	})
	rtC := simplert.New[key.Key256, kad.NodeID[key.Key256]](selfC, 2)
	schedC := event.NewSimpleScheduler(clk)
	endpointC := sim.NewEndpoint(selfC.NodeID(), schedC, router)
	servC := basicserver.NewBasicServer[multiaddr.Multiaddr](rtC, endpointC)
	err = endpointC.AddRequestHandler(protoID, nil, servC.HandleRequest)
	if err != nil {
		panic(err)
	}

	// connect peer A and B
	endpointA.MaybeAddToPeerstore(ctx, naddrB, peerstoreTTL)
	rtA.AddNode(selfB)
	endpointB.MaybeAddToPeerstore(ctx, naddrA, peerstoreTTL)
	rtB.AddNode(selfA)

	// connect peer B and C
	endpointB.MaybeAddToPeerstore(ctx, naddrC, peerstoreTTL)
	rtB.AddNode(selfC)
	endpointC.MaybeAddToPeerstore(ctx, naddrB, peerstoreTTL)
	rtC.AddNode(selfB)

	// create find peer request
	_, bin, _ := multibase.Decode(targetBytesID)
	target := libp2p.NewPeerID(peer.ID(bin))
	req := libp2p.FindPeerRequest(target)

	// dummy parameters
	handleResp := func(ctx context.Context, _ kad.NodeID[key.Key256],
		resp kad.Response[key.Key256, multiaddr.Multiaddr],
	) (bool, []kad.NodeID[key.Key256]) {
		peerids := make([]kad.NodeID[key.Key256], len(resp.CloserNodes()))
		for i, p := range resp.CloserNodes() {
			peerids[i] = p.(*libp2p.AddrInfo).PeerID()
		}
		return false, peerids
	}

	queryOpts := []sq.Option[key.Key256, multiaddr.Multiaddr]{
		sq.WithProtocolID[key.Key256, multiaddr.Multiaddr](protoID),
		sq.WithConcurrency[key.Key256, multiaddr.Multiaddr](1),
		sq.WithRequestTimeout[key.Key256, multiaddr.Multiaddr](5 * time.Second),
		sq.WithHandleResultsFunc[key.Key256, multiaddr.Multiaddr](handleResp),
		sq.WithRoutingTable[key.Key256, multiaddr.Multiaddr](rtA),
		sq.WithEndpoint[key.Key256, multiaddr.Multiaddr](endpointA),
		sq.WithScheduler[key.Key256, multiaddr.Multiaddr](schedA),
	}
	sq.NewSimpleQuery[key.Key256, multiaddr.Multiaddr](ctx, selfA.NodeID(), req, queryOpts...)

	// create simulator
	s := sim.NewLiteSimulator(clk)
	sim.AddSchedulers(s, schedA, schedB, schedC)
	// run simulation
	s.Run(ctx)
}

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

func main() {
	tp, err := tracerProvider("http://localhost:14268/api/traces")
	if err != nil {
		log.Fatal(err)
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}(ctx)

	queryTest(ctx)
}
