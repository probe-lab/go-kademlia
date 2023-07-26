package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

func TestClosestNodesIterContactsNodes(t *testing.T) {
	ctx := context.Background()

	cfg := DefaultClosestNodesIterConfig()

	target := key.Key8(0b00000011)
	neighbour := kadtest.NewID(key.Key8(0b00000010))

	knownNodes := []kad.NodeID[key.Key8]{
		neighbour,
	}

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the nearest node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// check that we are contacting the correct node
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, neighbour, st.NodeID)
}

func TestClosestNodesIterContactsNearest(t *testing.T) {
	ctx := context.Background()

	cfg := DefaultClosestNodesIterConfig()

	target := key.Key8(0b00000011)
	far := kadtest.NewID(key.Key8(0b11011011))
	near := kadtest.NewID(key.Key8(0b00000110))

	// ensure near is nearer to target than far is
	require.Less(t, target.Xor(near.Key()), target.Xor(far.Key()))

	// knownNodes are in "random" order with furthest before nearest
	knownNodes := []kad.NodeID[key.Key8]{
		far,
		near,
	}

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the nearest node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// check that we are contacting the nearest node first
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, near, st.NodeID)
}

func TestClosestNodesIterNoClosest(t *testing.T) {
	ctx := context.Background()

	cfg := DefaultClosestNodesIterConfig()

	target := key.Key8(0b00000011)

	// no known nodes to start with
	knownNodes := []kad.NodeID[key.Key8]{}

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the nearest node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterFinished[key.Key8]{}, state)
}

func TestClosestNodesIterContactsMultiple(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8
	c := kadtest.NewID(key.Key8(0b00010000)) // 16
	d := kadtest.NewID(key.Key8(0b00100000)) // 32

	// ensure the order of the known nodes
	require.True(t, target.Xor(a.Key()).Compare(target.Xor(b.Key())) == -1)
	require.True(t, target.Xor(b.Key()).Compare(target.Xor(c.Key())) == -1)
	require.True(t, target.Xor(c.Key()).Compare(target.Xor(d.Key())) == -1)

	// knownNodes are in "random" order
	knownNodes := []kad.NodeID[key.Key8]{b, c, a, d}

	cfg := DefaultClosestNodesIterConfig()
	cfg.Concurrency = len(knownNodes) - 1 // one less than the number of initial nodes

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the nearest node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, a, st.NodeID)

	// while the iter has capacity the iter should contact the next nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, b, st.NodeID)

	// while the iter has capacity the iter should contact the second nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, c, st.NodeID)

	// the iter should be at capacity
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingAtCapacity{}, state)
}
