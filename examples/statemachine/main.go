package main

import (
	"context"
	"fmt"

	"github.com/plprobelab/go-kademlia/kad"

	"github.com/plprobelab/go-kademlia/internal/kadtest"

	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/routing/simplert"
)

func main() {
	ctx := context.Background()

	nodes, mr := setupSimulation(ctx)

	kad := NewKademliaHandler[key.Key256, string](nodes[0], mr)

	ih := NewIpfsDht(kad)
	ih.Start(ctx)

	// A (ids[0]) is looking for D (ids[3])
	// A will first ask B, B will reply with C's address (and A's address)
	// A will then ask C, C will reply with D's address (and B's address)

	addr, err := ih.FindNode(context.Background(), nodes[3].NodeID())
	if err != nil {
		fmt.Printf("FindNode failed with error: %v\n", err)
		return
	}
	fmt.Printf("FindNode found address for: %s\n", addr.ID().String())
}

func setupSimulation(ctx context.Context) ([]*FakeNode[key.Key256, string], *MessageRouter[key.Key256, string]) {
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

	addrs := make([]kad.NodeInfo[key.Key256, string], nodeCount)
	for i := 0; i < nodeCount; i++ {
		addrs[i] = kadtest.NewAddr(ids[i], []string{})
	}

	nodes := make([]*FakeNode[key.Key256, string], nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = &FakeNode[key.Key256, string]{
			addr:      addrs[i],
			rt:        simplert.New(ids[i].Key(), 2),
			peerstore: make(map[kad.NodeID[key.Key256]]kad.NodeInfo[key.Key256, string]),
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
func connectNodes(ctx context.Context, a, b *FakeNode[key.Key256, string]) {
	// add b to a's peerstore and routing table
	a.AddNodeAddr(b.Addr())

	// add a to b's peerstore and routing table
	b.AddNodeAddr(a.Addr())
}

func traceReturnState(loc string, st State) {
	trace("  %s: returning state=%T", loc, st)
}

func traceCurrentState(loc string, st State) {
	trace("  %s: current state=%T", loc, st)
}

func trace(f string, args ...any) {
	fmt.Println(fmt.Sprintf(f, args...))
}

type QueryID uint64

const InvalidQueryID QueryID = 0

type RoutingUpdate any

type Event any
