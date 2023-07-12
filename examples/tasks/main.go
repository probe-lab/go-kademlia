package main

import (
	"context"
	"fmt"
	"time"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/kadaddr"
	"github.com/plprobelab/go-kademlia/network/address/kadid"
	"github.com/plprobelab/go-kademlia/routing/simplert"
	"github.com/plprobelab/go-kademlia/task"
)

func main() {
	ctx := context.Background()

	nodes, mr := setupSimulation(ctx)

	qp := NewQueryPool(nodes[0], mr)

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)

	target := nodes[3].Key()

	queryID, err := qp.AddQuery(ctx, target)
	if err != nil {
		panic(err.Error())
	}
	trace("Query id is %d", queryID)

	// continually advance the query pool state machine until we reach a terminal state
loop:
	for {

		state := qp.Advance(ctx)
		traceReturnState("main", state)
		switch st := state.(type) {
		case *QueryPoolWaiting:
			trace("Query pool is waiting for %d", st.QueryID)
		case *QueryPoolWaitingWithCapacity:
			trace("Query pool is waiting for one or more queries")
		case *QueryPoolFinished:
			trace("Query pool has finished query %d", st.QueryID)
			trace("Stats: %+v", st.Stats)
		case *QueryPoolTimeout:
			trace("Query pool has timed out query %d", st.QueryID)
		case *QueryPoolIdle:
			trace("Query pool has no further work, exiting this demo")
			break loop
		default:
			panic(fmt.Sprintf("unexpected state: %T", st))
		}
	}
}

const (
	keysize      = 1                                 // keysize in bytes
	peerstoreTTL = 10 * time.Minute                  // duration for which a peer is kept in the peerstore
	protoID      = address.ProtocolID("/test/1.0.0") // protocol ID for the test
)

func setupSimulation(ctx context.Context) ([]*FakeNode, *MessageRouter) {
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

	addrs := make([]address.NodeAddr, nodeCount)
	for i := 0; i < nodeCount; i++ {
		addrs[i] = kadaddr.NewKadAddr(ids[i], []string{})
	}

	nodes := make([]*FakeNode, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = &FakeNode{
			addr:      addrs[i],
			rt:        simplert.New(ids[i].KadKey, 2),
			peerstore: make(map[address.NodeID]address.NodeAddr),
		}
	}

	// A connects to B
	connectNodes(ctx, nodes[0], nodes[1])

	// B connects to C
	connectNodes(ctx, nodes[1], nodes[2])

	// C connects to D
	connectNodes(ctx, nodes[2], nodes[3])

	mr := NewMessageRouter(nodes)

	return nodes, mr
}

// connectNodes adds nodes to each other's peerstores and routing tables
func connectNodes(ctx context.Context, a, b *FakeNode) {
	// add n1 to n0's peerstore and routing table
	a.rt.AddPeer(ctx, b.NodeID())
	a.peerstore[b.NodeID()] = b.Addr()

	// add n0 to n1's peerstore and routing table
	b.rt.AddPeer(ctx, b.NodeID())
	b.peerstore[a.NodeID()] = a.Addr()
}

func traceReturnState(loc string, st task.State) {
	trace("  %s: returning state=%T", loc, st)
}

func traceCurrentState(loc string, st task.State) {
	trace("  %s: current state=%T", loc, st)
}

func trace(f string, args ...any) {
	fmt.Println(fmt.Sprintf(f, args...))
}
