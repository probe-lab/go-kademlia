package litesimulator

import (
	"strconv"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler/simplescheduler"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	sid "github.com/libp2p/go-libp2p-kad-dht/network/address/stringid"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint/fakeendpoint"
	"github.com/libp2p/go-libp2p-kad-dht/routingtable"
	"github.com/libp2p/go-libp2p-kad-dht/routingtable/simplert"
	"github.com/libp2p/go-libp2p-kad-dht/server"
	"github.com/libp2p/go-libp2p-kad-dht/server/basicserver"
)

var (
	bucketSize                    = 2
	protoID    address.ProtocolID = "/test/1.0.0"
)

func TestLiteSimulator(t *testing.T) {
	//ctx := context.Background()
	clk := clock.NewMock()

	router := fakeendpoint.NewFakeRouter()

	nNodes := 6
	nodes := make([]address.NodeID, nNodes)
	scheds := make([]scheduler.AwareScheduler, nNodes)
	rts := make([]routingtable.RoutingTable, nNodes)
	endpoints := make([]endpoint.SimEndpoint, nNodes)
	servers := make([]server.Server, nNodes)

	for i := 0; i < nNodes; i++ {
		nodes[i] = sid.NewStringID("node" + strconv.Itoa(i))
		scheds[i] = simplescheduler.NewSimpleScheduler(clk)
		rts[i] = simplert.NewSimpleRT(nodes[i].Key(), bucketSize)
		endpoints[i] = fakeendpoint.NewFakeEndpoint(nodes[i], scheds[i], router)
		servers[i] = basicserver.NewBasicServer(rts[i], endpoints[i], basicserver.WithNumberOfCloserPeersToSend(bucketSize))
		endpoints[i].AddRequestHandler(protoID, servers[i].HandleRequest)
	}
}
