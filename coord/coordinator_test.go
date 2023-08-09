package coord

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/events/scheduler"
	ss "github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/query"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/sim"
)

var (
	_ coordinatorInternalEvent = &eventUnroutablePeer[key.Key8]{}
	_ coordinatorInternalEvent = &eventMessageFailed[key.Key8]{}
	_ coordinatorInternalEvent = &eventMessageResponse[key.Key8, kadtest.StrAddr]{}
	_ coordinatorInternalEvent = &eventAddQuery[key.Key8, kadtest.StrAddr]{}
	_ coordinatorInternalEvent = &eventStopQuery[key.Key8]{}
)

func setupSimulation(t *testing.T, ctx context.Context) ([]kad.NodeInfo[key.Key8, kadtest.StrAddr], []*sim.Endpoint[key.Key8, kadtest.StrAddr], []kad.RoutingTable[key.Key8, kad.NodeID[key.Key8]], *sim.LiteSimulator) {
	// create node identifiers
	nodeCount := 4
	ids := make([]*kadtest.ID[key.Key8], nodeCount)
	ids[0] = kadtest.NewID(key.Key8(0x00))
	ids[1] = kadtest.NewID(key.Key8(0x01))
	ids[2] = kadtest.NewID(key.Key8(0x02))
	ids[3] = kadtest.NewID(key.Key8(0x03))

	// Kademlia trie:
	//     ^
	//    / \
	//   ^   ^
	//  A B C D

	addrs := make([]kad.NodeInfo[key.Key8, kadtest.StrAddr], nodeCount)
	for i := 0; i < nodeCount; i++ {
		addrs[i] = kadtest.NewInfo(ids[i], []kadtest.StrAddr{})
	}

	// create mock clock to control time
	clk := clock.NewMock()

	// create a fake router to virtually connect nodes
	router := sim.NewRouter[key.Key8, kadtest.StrAddr]()

	rts := make([]kad.RoutingTable[key.Key8, kad.NodeID[key.Key8]], len(addrs))
	eps := make([]*sim.Endpoint[key.Key8, kadtest.StrAddr], len(addrs))
	schedulers := make([]scheduler.AwareScheduler, len(addrs))
	servers := make([]*sim.Server[key.Key8, kadtest.StrAddr], len(addrs))

	for i := 0; i < len(addrs); i++ {
		i := i // :(
		// create a routing table, with bucket size 2
		rts[i] = simplert.New[key.Key8, kad.NodeID[key.Key8]](addrs[i].ID(), 2)
		// create a scheduler based on the mock clock
		schedulers[i] = ss.NewSimpleScheduler(clk)
		// create a fake endpoint for the node, communicating through the router
		eps[i] = sim.NewEndpoint[key.Key8, kadtest.StrAddr](addrs[i].ID(), schedulers[i], router)
		// create a server instance for the node
		servers[i] = sim.NewServer[key.Key8, kadtest.StrAddr](rts[i], eps[i], sim.DefaultServerConfig())
		// add the server request handler for protoID to the endpoint
		err := eps[i].AddRequestHandler(protoID, nil, servers[i].HandleFindNodeRequest)
		if err != nil {
			panic(err)
		}
	}

	// A connects to B
	connectNodes(t, addrs[0], addrs[1], eps[0], eps[1], rts[0], rts[1])

	// B connects to C
	connectNodes(t, addrs[1], addrs[2], eps[1], eps[2], rts[1], rts[2])

	// C connects to D
	connectNodes(t, addrs[2], addrs[3], eps[2], eps[3], rts[2], rts[3])

	// create a simulator, simulating [A, B, C, D]'s simulators
	siml := sim.NewLiteSimulator(clk)
	sim.AddSchedulers(siml, schedulers...)

	return addrs, eps, rts, siml
}

// connectNodes adds nodes to each other's peerstores and routing tables
func connectNodes(t *testing.T, n0, n1 kad.NodeInfo[key.Key8, kadtest.StrAddr], ep0, ep1 endpoint.Endpoint[key.Key8, kadtest.StrAddr],
	rt0, rt1 kad.RoutingTable[key.Key8, kad.NodeID[key.Key8]],
) {
	t.Helper()

	// add n1 to n0's peerstore and routing table
	t.Logf("connecting %s to %s", n0.ID(), n1.ID())
	ep0.MaybeAddToPeerstore(context.Background(), n1, peerstoreTTL)
	rt0.AddNode(n1.ID())

	// add n0 to n1's peerstore and routing table
	t.Logf("connecting %s to %s", n1.ID(), n0.ID())
	ep1.MaybeAddToPeerstore(context.Background(), n0, peerstoreTTL)
	rt1.AddNode(n0.ID())
}

const peerstoreTTL = 10 * time.Minute

var protoID = address.ProtocolID("/statemachine/1.0.0") // protocol ID for the test

// expectEventType selects on the event channel until an event of the expected type is sent.
func expectEventType(ctx context.Context, events <-chan KademliaEvent, expected KademliaEvent) (KademliaEvent, error) {
	for {
		select {
		case ev := <-events:
			if reflect.TypeOf(ev) == reflect.TypeOf(expected) {
				return ev, nil
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("test deadline exceeded")
		}
	}
}

// Ctx returns a Context and a CancelFunc. The context will be
// cancelled just before the test binary deadline (as
// specified by the -timeout flag when running the test). The
// CancelFunc may be called to cancel the context earlier than
// the deadline.
func Ctx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		deadline = time.Now().Add(time.Minute)
	} else {
		deadline = deadline.Add(-time.Second)
	}
	return context.WithDeadline(context.Background(), deadline)
}

func TestConfigValidate(t *testing.T) {
	t.Run("default is valid", func(t *testing.T) {
		cfg := DefaultConfig()
		require.NoError(t, cfg.Validate())
	})

	t.Run("clock is not nil", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Clock = nil
		require.Error(t, cfg.Validate())
	})

	t.Run("query concurrency positive", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.QueryConcurrency = 0
		require.Error(t, cfg.Validate())
		cfg.QueryConcurrency = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("query timeout positive", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.QueryTimeout = 0
		require.Error(t, cfg.Validate())
		cfg.QueryTimeout = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("request concurrency positive", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.RequestConcurrency = 0
		require.Error(t, cfg.Validate())
		cfg.QueryConcurrency = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("request timeout positive", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.RequestTimeout = 0
		require.Error(t, cfg.Validate())
		cfg.RequestTimeout = -1
		require.Error(t, cfg.Validate())
	})
}

func TestExhaustiveQuery(t *testing.T) {
	ctx, cancel := Ctx(t)
	defer cancel()

	nodes, eps, rts, siml := setupSimulation(t, ctx)

	clk := siml.Clock()

	ccfg := DefaultConfig()
	ccfg.Clock = clk
	ccfg.PeerstoreTTL = peerstoreTTL

	go func(ctx context.Context) {
		for {
			select {
			case <-time.After(10 * time.Millisecond):
				siml.Run(ctx)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)
	self := nodes[0].ID()
	c, err := NewCoordinator[key.Key8, kadtest.StrAddr](self, eps[0], rts[0], ccfg)
	if err != nil {
		log.Fatalf("unexpected error creating coordinator: %v", err)
	}
	siml.Add(c)
	events := c.Events()

	queryID := query.QueryID("query1")

	err = c.StartQuery(ctx, queryID, protoID, sim.NewRequest[key.Key8, kadtest.StrAddr](nodes[3].ID().Key()))
	if err != nil {
		t.Fatalf("failed to start query: %v", err)
	}

	// the query run by the coordinator should have received a response from nodes[1]
	ev, err := expectEventType(ctx, events, &KademliaOutboundQueryProgressedEvent[key.Key8, kadtest.StrAddr]{})
	require.NoError(t, err)

	tev := ev.(*KademliaOutboundQueryProgressedEvent[key.Key8, kadtest.StrAddr])
	require.Equal(t, nodes[1].ID(), tev.NodeID)
	require.Equal(t, queryID, tev.QueryID)

	// the query run by the coordinator should have received a response from nodes[2]
	ev, err = expectEventType(ctx, events, &KademliaOutboundQueryProgressedEvent[key.Key8, kadtest.StrAddr]{})
	require.NoError(t, err)

	tev = ev.(*KademliaOutboundQueryProgressedEvent[key.Key8, kadtest.StrAddr])
	require.Equal(t, nodes[2].ID(), tev.NodeID)
	require.Equal(t, queryID, tev.QueryID)

	// the query run by the coordinator should have received a response from nodes[3]
	ev, err = expectEventType(ctx, events, &KademliaOutboundQueryProgressedEvent[key.Key8, kadtest.StrAddr]{})
	require.NoError(t, err)

	tev = ev.(*KademliaOutboundQueryProgressedEvent[key.Key8, kadtest.StrAddr])
	require.Equal(t, nodes[3].ID(), tev.NodeID)
	require.Equal(t, queryID, tev.QueryID)

	// the query run by the coordinator should have completed
	ev, err = expectEventType(ctx, events, &KademliaOutboundQueryFinishedEvent{})
	require.NoError(t, err)

	require.IsType(t, &KademliaOutboundQueryFinishedEvent{}, ev)
	tevf := ev.(*KademliaOutboundQueryFinishedEvent)
	require.Equal(t, queryID, tevf.QueryID)
	require.Equal(t, 3, tevf.Stats.Requests)
	require.Equal(t, 3, tevf.Stats.Success)
	require.Equal(t, 0, tevf.Stats.Failure)
}

func TestRoutingUpdatedEventEmittedForCloserNodes(t *testing.T) {
	ctx, cancel := Ctx(t)
	defer cancel()

	nodes, eps, rts, siml := setupSimulation(t, ctx)

	clk := siml.Clock()

	ccfg := DefaultConfig()
	ccfg.Clock = clk
	ccfg.PeerstoreTTL = peerstoreTTL

	go func(ctx context.Context) {
		for {
			select {
			case <-time.After(10 * time.Millisecond):
				siml.Run(ctx)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)
	self := nodes[0].ID()
	c, err := NewCoordinator[key.Key8, kadtest.StrAddr](self, eps[0], rts[0], ccfg)
	if err != nil {
		log.Fatalf("unexpected error creating coordinator: %v", err)
	}
	siml.Add(c)
	events := c.Events()

	queryID := query.QueryID("query1")

	err = c.StartQuery(ctx, queryID, protoID, sim.NewRequest[key.Key8, kadtest.StrAddr](nodes[3].ID().Key()))
	if err != nil {
		t.Fatalf("failed to start query: %v", err)
	}

	// the query run by the coordinator should have received a response from nodes[1] with closer nodes
	// nodes[0] and nodes[2] which should trigger a routing table update
	ev, err := expectEventType(ctx, events, &KademliaRoutingUpdatedEvent[key.Key8, kadtest.StrAddr]{})
	require.NoError(t, err)

	tev := ev.(*KademliaRoutingUpdatedEvent[key.Key8, kadtest.StrAddr])
	require.Equal(t, nodes[2].ID(), tev.NodeInfo.ID())

	// no KademliaRoutingUpdatedEvent is sent for the self node

	// the query continues and should have received a response from nodes[2] with closer nodes
	// nodes[1] and nodes[3] which should trigger a routing table update
	ev, err = expectEventType(ctx, events, &KademliaRoutingUpdatedEvent[key.Key8, kadtest.StrAddr]{})
	require.NoError(t, err)

	tev = ev.(*KademliaRoutingUpdatedEvent[key.Key8, kadtest.StrAddr])
	require.Equal(t, nodes[3].ID(), tev.NodeInfo.ID())

	// the query run by the coordinator should have completed
	_, err = expectEventType(ctx, events, &KademliaOutboundQueryFinishedEvent{})
	require.NoError(t, err)
}
