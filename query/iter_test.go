package query

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
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

	// iterator is finished because there were no nodes to contat
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterFinished{}, state)

	st := state.(*StateNodeIterFinished)
	require.Equal(t, 0, st.Successes)
}

func TestClosestNodesIterWaitsAtCapacity(t *testing.T) {
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

func TestClosestNodesIterTimedOutNodeMakesCapacity(t *testing.T) {
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

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.NodeTimeout = 3 * time.Minute
	cfg.Concurrency = len(knownNodes) - 1 // one less than the number of initial nodes

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the nearest node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, a, st.NodeID)

	// advance time by one minute
	clk.Add(time.Minute)

	// while the iter has capacity the iter should contact the next nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, b, st.NodeID)

	// advance time by one minute
	clk.Add(time.Minute)

	// while the iter has capacity the iter should contact the second nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, c, st.NodeID)

	// advance time by one minute
	clk.Add(time.Minute)

	// the iter should be at capacity
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingAtCapacity{}, state)

	// advance time by another minute, now at 4 minutes, first node connection attempt should now time out
	clk.Add(time.Minute)

	// the first node request should have timed out, making capacity for the last node to attempt connection
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, d, st.NodeID)

	// advance time by another minute, now at 5 minutes, second node connection attempt should now time out
	clk.Add(time.Minute)

	// advancing now makes more capacity
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingWithCapacity{}, state)
}

func TestClosestNodesIterSuccessfulContactMakesCapacity(t *testing.T) {
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

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
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

	// notify iterator that first node was contacted successfully, now node d can be contacted
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{NodeID: a})
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, d, st.NodeID)

	// the iter should be at capacity again
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingAtCapacity{}, state)
}

func TestClosestNodesIterCloserNodesAreAddedToIteration(t *testing.T) {
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

	// one known node to start with
	knownNodes := []kad.NodeID[key.Key8]{d}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = 2

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the first node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, d, st.NodeID)

	// advancing reports iterator has capacity
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingWithCapacity{}, state)

	// notify iterator that first node was contacted successfully, with closer nodes
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID:      d,
		CloserNodes: []kad.NodeID[key.Key8]{b, a},
	})
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// iter should contact the next nearest uncontacted node
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, a, st.NodeID)
}

func TestClosestNodesIterCloserNodesIgnoresDuplicates(t *testing.T) {
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

	// one known node to start with
	knownNodes := []kad.NodeID[key.Key8]{d, a}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = 2

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the first node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, a, st.NodeID)

	// next the iter attempts to contact second nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, d, st.NodeID)

	// advancing reports iterator has no capacity
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingAtCapacity{}, state)

	// notify iterator that second node was contacted successfully, with closer nodes
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID:      d,
		CloserNodes: []kad.NodeID[key.Key8]{b, a},
	})
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// iter should contact the next nearest uncontacted node, which is b
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, b, st.NodeID)
}

func TestClosestNodesIterCancelFinishesIteration(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4

	// one known node to start with
	knownNodes := []kad.NodeID[key.Key8]{a}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = 2

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the first node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, a, st.NodeID)

	// cancel the iterator so it is now finished
	state = iter.Advance(ctx, &EventNodeIterCancel{})
	require.IsType(t, &StateNodeIterFinished{}, state)

	stf := state.(*StateNodeIterFinished)
	require.Equal(t, 0, stf.Successes)
}

func TestClosestNodesIterFinishedIgnoresLaterEvents(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8

	// one known node to start with
	knownNodes := []kad.NodeID[key.Key8]{b}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = 2

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the first node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, b, st.NodeID)

	// cancel the iterator so it is now finished
	state = iter.Advance(ctx, &EventNodeIterCancel{})
	require.IsType(t, &StateNodeIterFinished{}, state)

	// no successes
	stf := state.(*StateNodeIterFinished)
	require.Equal(t, 0, stf.Successes)

	// notify iterator that second node was contacted successfully, with closer nodes
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID:      b,
		CloserNodes: []kad.NodeID[key.Key8]{a},
	})

	// iterator remains finished
	require.IsType(t, &StateNodeIterFinished{}, state)

	// still no successes since contact message was after iterator had been cancelled
	stf = state.(*StateNodeIterFinished)
	require.Equal(t, 0, stf.Successes)
}

func TestClosestNodesIterIgnoresMessagesFromUnknownNodes(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8
	c := kadtest.NewID(key.Key8(0b00010000)) // 16

	// one known node to start with
	knownNodes := []kad.NodeID[key.Key8]{c}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = 2

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the first node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, c, st.NodeID)

	// notify iterator that second node was contacted successfully, with closer nodes
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID:      b,
		CloserNodes: []kad.NodeID[key.Key8]{a},
	})

	// iterator ignores message from unknown node
	require.IsType(t, &StateNodeIterWaitingWithCapacity{}, state)
}

func TestClosestNodesIterFinishesWhenNumResultsReached(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8
	c := kadtest.NewID(key.Key8(0b00010000)) // 16
	d := kadtest.NewID(key.Key8(0b00100000)) // 32

	// one known node to start with
	knownNodes := []kad.NodeID[key.Key8]{a, b, c, d}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = 4
	cfg.NumResults = 2

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// contact first node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, a, st.NodeID)

	// contact second node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, b, st.NodeID)

	// notify iterator that first node was contacted successfully
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID: a,
	})

	// iterator attempts to contact third node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, d, st.NodeID)

	// notify iterator that second node was contacted successfully
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID: b,
	})

	// iterator has finished since it contacted the NumResults closest peers
	require.IsType(t, &StateNodeIterFinished{}, state)
}

func TestClosestNodesIterContinuesUntilNumResultsReached(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8
	c := kadtest.NewID(key.Key8(0b00010000)) // 16

	// one known node to start with, the furthesr
	knownNodes := []kad.NodeID[key.Key8]{c}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = 4
	cfg.NumResults = 2

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// contact first node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st := state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, c, st.NodeID)

	// notify iterator that node was contacted successfully and tell it about
	// a closer one
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID:      c,
		CloserNodes: []kad.NodeID[key.Key8]{b},
	})

	// iterator attempts to contact second node
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, b, st.NodeID)

	// notify iterator that node was contacted successfully and tell it about
	// a closer one
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID:      b,
		CloserNodes: []kad.NodeID[key.Key8]{a},
	})

	// iterator has seen enough successful contacts but there are still
	// closer nodes that have not been contacted, so iterator attempts
	// to contact third node
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, a, st.NodeID)

	// notify iterator that second node was contacted successfully
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{
		NodeID: a,
	})

	// iterator has finished since it contacted the NumResults closest peers
	require.IsType(t, &StateNodeIterFinished{}, state)

	stf := state.(*StateNodeIterFinished)
	require.Equal(t, 2, stf.Successes)
}

func TestClosestNodesIterNotContactedMakesCapacity(t *testing.T) {
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

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
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

	// notify iterator that first node was not contacted, now node d can be contacted
	state = iter.Advance(ctx, &EventNodeIterNodeNotContacted[key.Key8]{NodeID: a})
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)
	st = state.(*StateNodeIterWaitingContact[key.Key8])
	require.Equal(t, d, st.NodeID)

	// the iter should be at capacity again
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingAtCapacity{}, state)
}

func TestClosestNodesIterAllNotContactedFinishes(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8
	c := kadtest.NewID(key.Key8(0b00010000)) // 16

	// knownNodes are in "random" order
	knownNodes := []kad.NodeID[key.Key8]{a, b, c}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = len(knownNodes) // allow all to be contacted at once

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the nearest node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// while the iter has capacity the iter should contact the next nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// while the iter has capacity the iter should contact the third nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// the iter should be at capacity
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingAtCapacity{}, state)

	// notify iterator that first node was not contacted
	state = iter.Advance(ctx, &EventNodeIterNodeNotContacted[key.Key8]{NodeID: a})
	require.IsType(t, &StateNodeIterWaitingWithCapacity{}, state)

	// notify iterator that second node was not contacted
	state = iter.Advance(ctx, &EventNodeIterNodeNotContacted[key.Key8]{NodeID: b})
	require.IsType(t, &StateNodeIterWaitingWithCapacity{}, state)

	// notify iterator that third node was not contacted
	state = iter.Advance(ctx, &EventNodeIterNodeNotContacted[key.Key8]{NodeID: c})

	// iterator has finished since it contacted all possible nodes
	require.IsType(t, &StateNodeIterFinished{}, state)

	st := state.(*StateNodeIterFinished)
	require.Equal(t, 0, st.Successes)
}

func TestClosestNodesIterAllContactedFinishes(t *testing.T) {
	ctx := context.Background()

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8
	c := kadtest.NewID(key.Key8(0b00010000)) // 16

	// knownNodes are in "random" order
	knownNodes := []kad.NodeID[key.Key8]{a, b, c}

	clk := clock.NewMock()

	cfg := DefaultClosestNodesIterConfig()
	cfg.Clock = clk
	cfg.Concurrency = len(knownNodes)    // allow all to be contacted at once
	cfg.NumResults = len(knownNodes) + 1 // one more than the size of the network

	iter := NewClosestNodesIter(target, knownNodes, cfg)

	// first thing the new iterator should do is contact the nearest node
	state := iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// while the iter has capacity the iter should contact the next nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// while the iter has capacity the iter should contact the third nearest node
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingContact[key.Key8]{}, state)

	// the iter should be at capacity
	state = iter.Advance(ctx, nil)
	require.IsType(t, &StateNodeIterWaitingAtCapacity{}, state)

	// notify iterator that first node was contacted successfully, but no closer nodes
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{NodeID: a})
	require.IsType(t, &StateNodeIterWaitingWithCapacity{}, state)

	// notify iterator that second node was contacted successfully, but no closer nodes
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{NodeID: b})
	require.IsType(t, &StateNodeIterWaitingWithCapacity{}, state)

	// notify iterator that third node was contacted successfully, but no closer nodes
	state = iter.Advance(ctx, &EventNodeIterNodeContacted[key.Key8]{NodeID: c})

	// iterator has finished since it contacted all possible nodes, even though it didn't
	// reach the desired NumResults
	require.IsType(t, &StateNodeIterFinished{}, state)

	st := state.(*StateNodeIterFinished)
	require.Equal(t, 3, st.Successes)
}
