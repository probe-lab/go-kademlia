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
)

func main() {
	ctx := context.Background()

	nodes, mr := setupSimulation(ctx)

	kad := NewKademliaHandler(nodes[0], mr)

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
	fmt.Printf("FindNode found address for: %s\n", addr.NodeID().String())
}

const (
	peerstoreTTL = 10 * time.Minute                  // duration for which a peer is kept in the peerstore
	protoID      = address.ProtocolID("/test/1.0.0") // protocol ID for the test
)

func setupSimulation(ctx context.Context) ([]*FakeNode[key.Key256], *MessageRouter[key.Key256]) {
	// create node identifiers
	nodeCount := 4
	ids := make([]*kadid.KadID[key.Key256], nodeCount)
	ids[0] = kadid.NewKadID(key.ZeroKey256())
	ids[1] = kadid.NewKadID(key.NewKey256(append(make([]byte, 31), 0x01)))
	ids[2] = kadid.NewKadID(key.NewKey256(append(make([]byte, 31), 0x02)))
	ids[3] = kadid.NewKadID(key.NewKey256(append(make([]byte, 31), 0x03)))

	// Kademlia trie:
	//     ^
	//    / \
	//   ^   ^
	//  A B C D

	addrs := make([]address.NodeAddr[key.Key256], nodeCount)
	for i := 0; i < nodeCount; i++ {
		addrs[i] = kadaddr.NewKadAddr(ids[i], []string{})
	}

	nodes := make([]*FakeNode[key.Key256], nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = &FakeNode[key.Key256]{
			addr:      addrs[i],
			rt:        simplert.New(ids[i].Key(), 2),
			peerstore: make(map[address.NodeID[key.Key256]]address.NodeAddr[key.Key256]),
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
func connectNodes(ctx context.Context, a, b *FakeNode[key.Key256]) {
	// add b to a's peerstore and routing table
	a.AddNodeAddr(ctx, b.Addr())

	// add a to b's peerstore and routing table
	b.AddNodeAddr(ctx, a.Addr())
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
