package query

import (
	"context"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"

	"github.com/plprobelab/go-kademlia/internal/kadtest"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
)

func TestPoolConfigValidate(t *testing.T) {
	t.Run("default is valid", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		require.NoError(t, cfg.Validate())
	})

	t.Run("clock is not nil", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		cfg.Clock = nil
		require.Error(t, cfg.Validate())
	})

	t.Run("concurrency positive", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		cfg.Concurrency = 0
		require.Error(t, cfg.Validate())
		cfg.Concurrency = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("timeout positive", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		cfg.Timeout = 0
		require.Error(t, cfg.Validate())
		cfg.Timeout = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("replication positive", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		cfg.Replication = 0
		require.Error(t, cfg.Validate())
		cfg.Replication = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("query concurrency positive", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		cfg.QueryConcurrency = 0
		require.Error(t, cfg.Validate())
		cfg.QueryConcurrency = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("request timeout positive", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		cfg.RequestTimeout = 0
		require.Error(t, cfg.Validate())
		cfg.RequestTimeout = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("queue capacity positive", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		cfg.QueueCapacity = 0
		require.Error(t, cfg.Validate())
		cfg.QueueCapacity = -1
		require.Error(t, cfg.Validate())
	})
}

func TestPoolStartsIdle(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	cfg := DefaultPoolConfig()
	cfg.Clock = clk

	self := kadtest.NewID(key.Key8(0))
	p, err := NewPool[key.Key8, kadtest.StrAddr](self, cfg)
	require.NoError(t, err)

	state := p.Advance(ctx)
	require.IsType(t, &StatePoolIdle{}, state)
}

func TestPoolStopWhenNoQueries(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	cfg := DefaultPoolConfig()
	cfg.Clock = clk

	self := kadtest.NewID(key.Key8(0))
	p, err := NewPool[key.Key8, kadtest.StrAddr](self, cfg)
	require.NoError(t, err)

	p.Enqueue(ctx, &EventPoolStopQuery{})
	state := p.Advance(ctx)
	require.IsType(t, &StatePoolIdle{}, state)
}

func TestPoolAddQueryStartsIfCapacity(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	cfg := DefaultPoolConfig()
	cfg.Clock = clk

	self := kadtest.NewID(key.Key8(0))
	p, err := NewPool[key.Key8, kadtest.StrAddr](self, cfg)
	require.NoError(t, err)

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4

	msg := kadtest.NewRequest("1", target)
	queryID := QueryID("test")
	protocolID := address.ProtocolID("testprotocol")

	// first thing the new pool should do is start the query
	p.Enqueue(ctx, &EventPoolAddQuery[key.Key8, kadtest.StrAddr]{
		QueryID:           queryID,
		Target:            target,
		ProtocolID:        protocolID,
		Message:           msg,
		KnownClosestNodes: []kad.NodeID[key.Key8]{a},
	})
	state := p.Advance(ctx)
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)

	// the query should attempt to contact the node it was given
	st := state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])

	// the query should be the one just added
	require.Equal(t, queryID, st.QueryID)

	// the query should attempt to contact the node it was given
	require.Equal(t, a, st.NodeID)

	// with the correct protocol ID
	require.Equal(t, protocolID, st.ProtocolID)

	// with the correct message
	require.Equal(t, msg, st.Message)

	// now the pool reports that it is waiting
	state = p.Advance(ctx)
	require.IsType(t, &StatePoolWaitingWithCapacity{}, state)
}

func TestPoolMessageResponse(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	cfg := DefaultPoolConfig()
	cfg.Clock = clk

	self := kadtest.NewID(key.Key8(0))
	p, err := NewPool[key.Key8, kadtest.StrAddr](self, cfg)
	require.NoError(t, err)

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4

	msg := kadtest.NewRequest("1", target)
	queryID := QueryID("test")
	protocolID := address.ProtocolID("testprotocol")

	// first thing the new pool should do is start the query
	p.Enqueue(ctx, &EventPoolAddQuery[key.Key8, kadtest.StrAddr]{
		QueryID:           queryID,
		Target:            target,
		ProtocolID:        protocolID,
		Message:           msg,
		KnownClosestNodes: []kad.NodeID[key.Key8]{a},
	})
	state := p.Advance(ctx)
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)

	// the query should attempt to contact the node it was given
	st := state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID, st.QueryID)
	require.Equal(t, a, st.NodeID)

	// notify query that node was contacted successfully, but no closer nodes
	p.Enqueue(ctx, &EventPoolMessageResponse[key.Key8, kadtest.StrAddr]{
		QueryID: queryID,
		NodeID:  a,
	})
	state = p.Advance(ctx)

	// pool should respond that query has finished
	require.IsType(t, &StatePoolQueryFinished{}, state)

	stf := state.(*StatePoolQueryFinished)
	require.Equal(t, queryID, stf.QueryID)
	require.Equal(t, 1, stf.Stats.Requests)
	require.Equal(t, 1, stf.Stats.Success)
}

func TestPoolPrefersRunningQueriesOverNewOnes(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	cfg := DefaultPoolConfig()
	cfg.Clock = clk
	cfg.Concurrency = 2 // allow two queries to run concurrently

	self := kadtest.NewID(key.Key8(0))
	p, err := NewPool[key.Key8, kadtest.StrAddr](self, cfg)
	require.NoError(t, err)

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4
	b := kadtest.NewID(key.Key8(0b00001000)) // 8
	c := kadtest.NewID(key.Key8(0b00010000)) // 16
	d := kadtest.NewID(key.Key8(0b00100000)) // 32

	msg1 := kadtest.NewRequest("1", target)
	queryID1 := QueryID("1")

	protocolID := address.ProtocolID("testprotocol")

	// Add the first query
	p.Enqueue(ctx, &EventPoolAddQuery[key.Key8, kadtest.StrAddr]{
		QueryID:           queryID1,
		Target:            target,
		ProtocolID:        protocolID,
		Message:           msg1,
		KnownClosestNodes: []kad.NodeID[key.Key8]{a, b, c, d},
	})
	state := p.Advance(ctx)
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)

	// the first query should attempt to contact the node it was given
	st := state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID1, st.QueryID)
	require.Equal(t, a, st.NodeID)

	msg2 := kadtest.NewRequest("2", target)
	queryID2 := QueryID("2")

	// Add the second query
	p.Enqueue(ctx, &EventPoolAddQuery[key.Key8, kadtest.StrAddr]{
		QueryID:           queryID2,
		Target:            target,
		ProtocolID:        protocolID,
		Message:           msg2,
		KnownClosestNodes: []kad.NodeID[key.Key8]{a, b, c, d},
	})
	state = p.Advance(ctx)

	// the first query should continue its operation in preference to starting the new query
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)
	st = state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID1, st.QueryID)
	require.Equal(t, b, st.NodeID)

	// advance the pool again, the first query should continue its operation in preference to starting the new query
	state = p.Advance(ctx)
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)
	st = state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID1, st.QueryID)
	require.Equal(t, c, st.NodeID)

	// advance the pool again, the first query is at capacity so the second query can start
	state = p.Advance(ctx)
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)
	st = state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID2, st.QueryID)
	require.Equal(t, a, st.NodeID)

	// notify first query that node was contacted successfully, but no closer nodes
	p.Enqueue(ctx, &EventPoolMessageResponse[key.Key8, kadtest.StrAddr]{
		QueryID: queryID1,
		NodeID:  a,
	})
	state = p.Advance(ctx)

	// first query starts a new message request
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)
	st = state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID1, st.QueryID)
	require.Equal(t, d, st.NodeID)

	// notify first query that next node was contacted successfully, but no closer nodes
	p.Enqueue(ctx, &EventPoolMessageResponse[key.Key8, kadtest.StrAddr]{
		QueryID: queryID1,
		NodeID:  b,
	})
	state = p.Advance(ctx)

	// first query is out of nodes to try so second query can proceed
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)
	st = state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID2, st.QueryID)
	require.Equal(t, b, st.NodeID)
}

func TestPoolRespectsConcurrency(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	cfg := DefaultPoolConfig()
	cfg.Clock = clk
	cfg.Concurrency = 2      // allow two queries to run concurrently
	cfg.QueryConcurrency = 1 // allow each query to have a single request in flight

	self := kadtest.NewID(key.Key8(0))
	p, err := NewPool[key.Key8, kadtest.StrAddr](self, cfg)
	require.NoError(t, err)

	target := key.Key8(0b00000001)
	a := kadtest.NewID(key.Key8(0b00000100)) // 4

	msg1 := kadtest.NewRequest("1", target)
	queryID1 := QueryID("1")

	protocolID := address.ProtocolID("testprotocol")

	// Add the first query
	p.Enqueue(ctx, &EventPoolAddQuery[key.Key8, kadtest.StrAddr]{
		QueryID:           queryID1,
		Target:            target,
		ProtocolID:        protocolID,
		Message:           msg1,
		KnownClosestNodes: []kad.NodeID[key.Key8]{a},
	})
	state := p.Advance(ctx)
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)

	// the first query should attempt to contact the node it was given
	st := state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID1, st.QueryID)
	require.Equal(t, a, st.NodeID)

	msg2 := kadtest.NewRequest("2", target)
	queryID2 := QueryID("2")

	// Add the second query
	p.Enqueue(ctx, &EventPoolAddQuery[key.Key8, kadtest.StrAddr]{
		QueryID:           queryID2,
		Target:            target,
		ProtocolID:        protocolID,
		Message:           msg2,
		KnownClosestNodes: []kad.NodeID[key.Key8]{a},
	})
	state = p.Advance(ctx)

	// the second query should start since the first query has a request in flight
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)
	st = state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID2, st.QueryID)
	require.Equal(t, a, st.NodeID)

	msg3 := kadtest.NewRequest("3", target)
	queryID3 := QueryID("3")

	// Add a third query
	p.Enqueue(ctx, &EventPoolAddQuery[key.Key8, kadtest.StrAddr]{
		QueryID:           queryID3,
		Target:            target,
		ProtocolID:        protocolID,
		Message:           msg3,
		KnownClosestNodes: []kad.NodeID[key.Key8]{a},
	})
	state = p.Advance(ctx)

	// the third query should wait since the pool has reached maximum concurrency
	require.IsType(t, &StatePoolWaitingAtCapacity{}, state)

	// notify first query that next node was contacted successfully, but no closer nodes
	p.Enqueue(ctx, &EventPoolMessageResponse[key.Key8, kadtest.StrAddr]{
		QueryID: queryID1,
		NodeID:  a,
	})
	state = p.Advance(ctx)

	// first query is out of nodes so it has finished
	require.IsType(t, &StatePoolQueryFinished{}, state)
	stf := state.(*StatePoolQueryFinished)
	require.Equal(t, queryID1, stf.QueryID)

	// advancing pool again allows query 3 to start
	state = p.Advance(ctx)
	require.IsType(t, &StatePoolQueryMessage[key.Key8, kadtest.StrAddr]{}, state)
	st = state.(*StatePoolQueryMessage[key.Key8, kadtest.StrAddr])
	require.Equal(t, queryID3, st.QueryID)
	require.Equal(t, a, st.NodeID)
}
