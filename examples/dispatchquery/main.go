package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multibase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	ss "github.com/libp2p/go-libp2p-kad-dht/events/scheduler/simplescheduler"
	"github.com/libp2p/go-libp2p-kad-dht/events/simulator"
	"github.com/libp2p/go-libp2p-kad-dht/events/simulator/litesimulator"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/addrinfo"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/peerid"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint/fakeendpoint"
	"github.com/libp2p/go-libp2p-kad-dht/network/message"
	"github.com/libp2p/go-libp2p-kad-dht/network/message/ipfsv1"
	sq "github.com/libp2p/go-libp2p-kad-dht/query/simplequery"
	"github.com/libp2p/go-libp2p-kad-dht/routingtable/simplert"
	"github.com/libp2p/go-libp2p-kad-dht/server/basicserver"
	"github.com/libp2p/go-libp2p-kad-dht/util"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	peerstoreTTL = 10 * time.Minute
	protoID      = "/ipfs/kad/1.0.0"
)

var (
	targetBytesID = "mACQIARIgp9PBu+JuU8aicuW8xT+Oa08OntMyqdLbfQtOplAHlME"
)

func queryTest(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "queryTest")
	defer span.End()

	clk := clock.NewMock()

	router := fakeendpoint.NewFakeRouter()

	// create peer A
	pidA, err := peer.Decode("12BooooALPHA")
	if err != nil {
		panic(err)
	}
	selfA := &peerid.PeerID{ID: pidA} // peer.ID is necessary for ipfskadv1 message format
	addrA := multiaddr.StringCast("/ip4/1.1.1.1/tcp/4001/")
	var naddrA address.NodeID = addrinfo.NewAddrInfo(peer.AddrInfo{ID: selfA.ID,
		Addrs: []multiaddr.Multiaddr{addrA}})
	rtA := simplert.NewSimpleRT(selfA.Key(), 2)
	schedA := ss.NewSimpleScheduler(clk)
	endpointA := fakeendpoint.NewFakeEndpoint(selfA, schedA, router)
	servA := basicserver.NewBasicServer(rtA, endpointA)
	endpointA.AddRequestHandler(protoID, servA.HandleRequest)

	// create peer B
	pidB, err := peer.Decode("12BoooooBETA")
	if err != nil {
		panic(err)
	}
	selfB := &peerid.PeerID{ID: pidB}
	addrB := multiaddr.StringCast("/ip4/2.2.2.2/tcp/4001/")
	var naddrB address.NodeID = addrinfo.NewAddrInfo(peer.AddrInfo{ID: selfB.ID,
		Addrs: []multiaddr.Multiaddr{addrB}})
	rtB := simplert.NewSimpleRT(selfB.Key(), 2)
	schedB := ss.NewSimpleScheduler(clk)
	endpointB := fakeendpoint.NewFakeEndpoint(selfB, schedB, router)
	servB := basicserver.NewBasicServer(rtB, endpointB)
	endpointB.AddRequestHandler(protoID, servB.HandleRequest)

	// create peer C
	pidC, err := peer.Decode("12BooooGAMMA")
	if err != nil {
		panic(err)
	}
	selfC := &peerid.PeerID{ID: pidC}
	addrC := multiaddr.StringCast("/ip4/3.3.3.3/tcp/4001/")
	var naddrC address.NodeID = addrinfo.NewAddrInfo(peer.AddrInfo{ID: selfC.ID,
		Addrs: []multiaddr.Multiaddr{addrC}})
	rtC := simplert.NewSimpleRT(selfC.Key(), 2)
	schedC := ss.NewSimpleScheduler(clk)
	endpointC := fakeendpoint.NewFakeEndpoint(selfC, schedC, router)
	servC := basicserver.NewBasicServer(rtC, endpointC)
	endpointC.AddRequestHandler(protoID, servC.HandleRequest)

	// connect peer A and B
	endpointA.MaybeAddToPeerstore(ctx, naddrB, peerstoreTTL)
	rtA.AddPeer(ctx, selfB)
	endpointB.MaybeAddToPeerstore(ctx, naddrA, peerstoreTTL)
	rtB.AddPeer(ctx, selfA)

	// connect peer B and C
	endpointB.MaybeAddToPeerstore(ctx, naddrC, peerstoreTTL)
	rtB.AddPeer(ctx, selfC)
	endpointC.MaybeAddToPeerstore(ctx, naddrB, peerstoreTTL)
	rtC.AddPeer(ctx, selfB)

	// create find peer request
	_, bin, _ := multibase.Decode(targetBytesID)
	target := peerid.NewPeerID(peer.ID(bin))
	req := ipfsv1.FindPeerRequest(target)
	resp := &ipfsv1.Message{}

	// dummy parameters
	handleResp := func(ctx context.Context, _ address.NodeID,
		resp message.MinKadResponseMessage) (bool, []address.NodeID) {
		fmt.Println(resp.CloserNodes())
		peerids := make([]address.NodeID, len(resp.CloserNodes()))
		for i, p := range resp.CloserNodes() {
			peerids[i] = p.(*addrinfo.AddrInfo).PeerID()
		}
		return false, peerids
	}
	sq.NewSimpleQuery(ctx, target.Key(), protoID, req, resp, 1, time.Second, endpointA,
		rtA, schedA, handleResp)

	// create simulator
	sim := litesimulator.NewLiteSimulator(clk)
	simulator.AddPeers(sim, schedA, schedB, schedC)
	// run simulation
	sim.Run(ctx)
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
