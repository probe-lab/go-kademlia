package simplequery

import (
	"context"
	"testing"

	"github.com/benbjohnson/clock"
	ss "github.com/libp2p/go-libp2p-kad-dht/events/scheduler/simplescheduler"
	si "github.com/libp2p/go-libp2p-kad-dht/network/address/stringid"
)

func TestTrivialQuery(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	//dispatcher := sd.NewSimpleDispatcher(clk)
	node0 := si.StringID("node0")
	node1 := si.StringID("node1")
	sched0 := ss.NewSimpleScheduler(clk)
	sched1 := ss.NewSimpleScheduler(clk)
	//fendpoint0 := fe.NewFakeEndpoint(node0, dispatcher)
	//fendpoint1 := fe.NewFakeEndpoint(node1, dispatcher)

	_ = sched0
	_ = sched1
	_ = node0
	_ = node1
	_ = ctx
}
