package simplequery

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	ss "github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/network/address"
	si "github.com/plprobelab/go-kademlia/network/address/stringid"
	fe "github.com/plprobelab/go-kademlia/network/endpoint/fakeendpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	sm "github.com/plprobelab/go-kademlia/network/message/simmessage"
	"github.com/plprobelab/go-kademlia/routingtable/simplert"
	"github.com/plprobelab/go-kademlia/server/basicserver"
)

func TestTrivialQuery(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	protoID := address.ProtocolID("/test/1.0.0")

	router := fe.NewFakeRouter()
	node0 := si.StringID("node0")
	node1 := si.StringID("node1")
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	fendpoint0 := fe.NewFakeEndpoint(node0, sched0, router)
	fendpoint1 := fe.NewFakeEndpoint(node1, sched1, router)
	rt0 := simplert.NewSimpleRT(node0.Key(), 1)
	rt1 := simplert.NewSimpleRT(node1.Key(), 1)

	server1 := basicserver.NewBasicServer(rt1, fendpoint1)
	fendpoint1.AddRequestHandler(protoID, &sm.SimMessage{}, server1.HandleRequest)

	req := sm.NewSimRequest(node1.Key())
	handleResFn := func(context.Context, address.NodeID,
		message.MinKadResponseMessage) (bool, []address.NodeID) {
		return true, []address.NodeID{node1}
	}
	queryOpts := []Option{
		WithProtocolID(protoID),
		WithConcurrency(1),
		WithRequestTimeout(time.Second),
		WithHandleResultsFunc(handleResFn),
		WithRoutingTable(rt0),
		WithEndpoint(fendpoint0),
		WithScheduler(sched0),
	}
	NewSimpleQuery(ctx, req, queryOpts...)
}
