package routing

import (
	"container/heap"
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"

	"github.com/probe-lab/go-kademlia/internal/kadtest"
	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
	"github.com/probe-lab/go-kademlia/routing/simplert"
)

var _ heap.Interface = (*nodeValuePendingList[key.Key8])(nil)

type unaddressedNodeInfo[K kad.Key[K], A kad.Address[A]] struct {
	NodeID kad.NodeID[K]
}

func (u unaddressedNodeInfo[K, A]) ID() kad.NodeID[K] { return u.NodeID }
func (u unaddressedNodeInfo[K, A]) Addresses() []A    { return nil }

func TestProbeConfigValidate(t *testing.T) {
	t.Run("default is valid", func(t *testing.T) {
		cfg := DefaultProbeConfig()
		require.NoError(t, cfg.Validate())
	})

	t.Run("clock is not nil", func(t *testing.T) {
		cfg := DefaultProbeConfig()
		cfg.Clock = nil
		require.Error(t, cfg.Validate())
	})

	t.Run("timeout positive", func(t *testing.T) {
		cfg := DefaultProbeConfig()
		cfg.Timeout = 0
		require.Error(t, cfg.Validate())
		cfg.Timeout = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("request concurrency positive", func(t *testing.T) {
		cfg := DefaultProbeConfig()
		cfg.Concurrency = 0
		require.Error(t, cfg.Validate())
		cfg.Concurrency = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("revisit interval positive", func(t *testing.T) {
		cfg := DefaultProbeConfig()
		cfg.CheckInterval = 0
		require.Error(t, cfg.Validate())
		cfg.CheckInterval = -1
		require.Error(t, cfg.Validate())
	})
}

func TestProbeStartsIdle(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	cfg := DefaultProbeConfig()
	cfg.Clock = clk

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)

	bs, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	state := bs.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeIdle{}, state)
}

func TestProbeAddChecksPresenceInRoutingTable(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute

	// Set concurrency to allow one check to run
	cfg.Concurrency = 1

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// Add node that isn't in routing table
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by one revisit interval
	clk.Add(cfg.CheckInterval)

	// remains idle since probes aren't run unless node in routing table
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeIdle{}, state)
}

func TestProbeAddStartsCheckIfCapacity(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute

	// Set concurrency to allow one check to run
	cfg.Concurrency = 1

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// after adding first node the probe should be idle since the
	// connectivity check will be scheduled for the future
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// remains idle
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by one revisit interval
	clk.Add(cfg.CheckInterval)
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the probe state machine should attempt to contact the next node
	st := state.(*StateProbeConnectivityCheck[key.Key8])

	// the connectivity check should be for the right node
	require.True(t, key.Equal(key.Key8(4), st.NodeID.Key()))
}

func TestProbeAddManyStartsChecksIfCapacity(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute

	// Set concurrency lower than the number of nodes
	cfg.Concurrency = 2

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))
	rt.AddNode(kadtest.NewID(key.Key8(3)))
	rt.AddNode(kadtest.NewID(key.Key8(2)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// after adding first node the probe should be idle since the
	// connectivity check will be scheduled for the future
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// after adding second node the probe should still be idle since the
	// connectivity check will be scheduled for the future
	state = sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(3)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// after adding third node the probe should still be idle since the
	// connectivity check will be scheduled for the future
	state = sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(2)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by one revisit interval
	clk.Add(cfg.CheckInterval)

	// Poll the state machine, it should now attempt to contact a node
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the connectivity check should be for the right node
	st := state.(*StateProbeConnectivityCheck[key.Key8])
	require.True(t, key.Equal(key.Key8(4), st.NodeID.Key()))

	// Poll the state machine, it should now attempt to contact another node
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the connectivity check should be for the right node
	st = state.(*StateProbeConnectivityCheck[key.Key8])
	require.True(t, key.Equal(key.Key8(2), st.NodeID.Key()))

	// Poll the state machine, it should now be at capacity
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeWaitingAtCapacity{}, state)
}

func TestProbeAddReportsCapacity(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute

	// Set concurrency to allow more than one check to run
	cfg.Concurrency = 2

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// after adding first node the probe should be idle since the
	// connectivity check will be scheduled for the future
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// remains idle
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by one revisit interval
	clk.Add(cfg.CheckInterval)
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the probe state machine should attempt to contact the next node
	st := state.(*StateProbeConnectivityCheck[key.Key8])

	// the connectivity check should be for the right node
	require.True(t, key.Equal(key.Key8(4), st.NodeID.Key()))

	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeWaitingWithCapacity{}, state)
}

func TestProbeRemoveDeletesNodeValue(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute

	// Set concurrency to allow more than one check to run
	cfg.Concurrency = 2

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// after adding first node the probe should be idle since the
	// connectivity check will be scheduled for the future
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// remove the node
	state = sm.Advance(ctx, &EventProbeRemove[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})

	// state indicate that node failed
	require.IsType(t, &StateProbeNodeFailure[key.Key8]{}, state)

	// advance time by one revisit interval
	clk.Add(cfg.CheckInterval)

	// state remains idle since there are no nodes to probe
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeIdle{}, state)
}

func TestNodeValueList(t *testing.T) {
	t.Run("put new", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now(),
		}

		l.Put(nv)

		got, found := l.Get(kadtest.NewID(key.Key8(4)))
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), key.Key8(4)))
	})

	t.Run("put replace before", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now(),
		}

		l.Put(nv1)

		nv2 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now().Add(-time.Minute),
		}
		l.Put(nv2)

		got, found := l.Get(kadtest.NewID(key.Key8(4)))
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), key.Key8(4)))
		require.Equal(t, nv2.NextCheckDue, got.NextCheckDue)
	})

	t.Run("put replace after", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now(),
		}

		l.Put(nv1)

		nv2 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now().Add(time.Minute),
		}
		l.Put(nv2)

		got, found := l.Get(kadtest.NewID(key.Key8(4)))
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), key.Key8(4)))
		require.Equal(t, nv2.NextCheckDue, got.NextCheckDue)
	})

	t.Run("remove existing", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now(),
		}

		l.Put(nv)

		require.Equal(t, 1, l.PendingCount())
		require.Equal(t, 1, l.NodeCount())

		_, found := l.Get(kadtest.NewID(key.Key8(4)))
		require.True(t, found)

		l.Remove(kadtest.NewID(key.Key8(4)))
		_, found = l.Get(kadtest.NewID(key.Key8(4)))
		require.False(t, found)

		require.Equal(t, 0, l.PendingCount())
		require.Equal(t, 0, l.NodeCount())
	})

	t.Run("remove not-existing", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now(),
		}

		l.Put(nv)

		l.Remove(kadtest.NewID(key.Key8(5)))
		_, found := l.Get(kadtest.NewID(key.Key8(4)))
		require.True(t, found)
	})

	t.Run("next empty list", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		got, found := l.PeekNext(clk.Now())
		require.False(t, found)
		require.Nil(t, got)
	})

	t.Run("next one entry", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now(),
		}
		l.Put(nv)

		got, found := l.PeekNext(clk.Now())
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), key.Key8(4)))
	})

	t.Run("next sorts by next check due", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(5)),
			NextCheckDue: clk.Now().Add(-time.Minute),
		}
		l.Put(nv1)
		nv2 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now().Add(-2 * time.Minute),
		}
		l.Put(nv2)

		got, found := l.PeekNext(clk.Now())
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), nv2.NodeID.Key()))

		nv2.NextCheckDue = clk.Now()
		l.Put(nv2)

		got, found = l.PeekNext(clk.Now())
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), nv1.NodeID.Key()))
	})

	t.Run("next sorts by cpl descending after time", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(5)),
			Cpl:          1,
			NextCheckDue: clk.Now().Add(-time.Minute),
		}
		l.Put(nv1)
		nv2 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			Cpl:          2,
			NextCheckDue: clk.Now().Add(-time.Minute),
		}
		l.Put(nv2)

		got, found := l.PeekNext(clk.Now())
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), nv2.NodeID.Key()))

		nv2.NextCheckDue = clk.Now()
		l.Put(nv2)

		got, found = l.PeekNext(clk.Now())
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), nv1.NodeID.Key()))
	})

	t.Run("next not due", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(5)),
			NextCheckDue: clk.Now().Add(time.Minute),
		}
		l.Put(nv1)
		nv2 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now().Add(2 * time.Minute),
		}
		l.Put(nv2)

		got, found := l.PeekNext(clk.Now())
		require.False(t, found)
		require.Nil(t, got)
	})

	t.Run("mark ongoing", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(5)),
			NextCheckDue: clk.Now().Add(time.Minute),
		}
		l.Put(nv1)
		require.Equal(t, 1, l.PendingCount())
		require.Equal(t, 0, l.OngoingCount())
		require.Equal(t, 1, l.NodeCount())

		l.MarkOngoing(kadtest.NewID(key.Key8(5)), clk.Now().Add(time.Minute))
		require.Equal(t, 0, l.PendingCount())
		require.Equal(t, 1, l.OngoingCount())
		require.Equal(t, 1, l.NodeCount())
	})

	t.Run("mark ongoing changes next", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(5)),
			NextCheckDue: clk.Now().Add(-2 * time.Minute),
		}
		l.Put(nv1)

		nv2 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now().Add(-1 * time.Minute),
		}
		l.Put(nv2)

		require.Equal(t, 2, l.PendingCount())
		require.Equal(t, 0, l.OngoingCount())
		require.Equal(t, 2, l.NodeCount())

		// nv1 is the next node due
		got, found := l.PeekNext(clk.Now())
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), nv1.NodeID.Key()))

		l.MarkOngoing(nv1.NodeID, clk.Now().Add(time.Minute))
		require.Equal(t, 1, l.PendingCount())
		require.Equal(t, 1, l.OngoingCount())
		require.Equal(t, 2, l.NodeCount())

		// nv2 is now the next node due
		got, found = l.PeekNext(clk.Now())
		require.True(t, found)
		require.True(t, key.Equal(got.NodeID.Key(), nv2.NodeID.Key()))
	})

	t.Run("put removes from ongoing", func(t *testing.T) {
		t.Parallel()

		clk := clock.NewMock()
		l := NewNodeValueList[key.Key8]()
		nv1 := &nodeValue[key.Key8]{
			NodeID:       kadtest.NewID(key.Key8(4)),
			NextCheckDue: clk.Now(),
		}
		l.Put(nv1)

		require.Equal(t, 1, l.PendingCount())
		require.Equal(t, 0, l.OngoingCount())
		require.Equal(t, 1, l.NodeCount())

		l.MarkOngoing(nv1.NodeID, clk.Now().Add(time.Minute))

		require.Equal(t, 0, l.PendingCount())
		require.Equal(t, 1, l.OngoingCount())
		require.Equal(t, 1, l.NodeCount())

		l.Put(nv1)

		require.Equal(t, 1, l.PendingCount())
		require.Equal(t, 0, l.OngoingCount())
		require.Equal(t, 1, l.NodeCount())
	})
}

func TestProbeMessageResponse(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute

	// Set concurrency to allow more than one check to run
	cfg.Concurrency = 2

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// after adding first node the probe should be idle since the
	// connectivity check will be scheduled for the future
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by one revisit interval
	clk.Add(cfg.CheckInterval)
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the probe state machine should attempt to contact the next node
	st := state.(*StateProbeConnectivityCheck[key.Key8])

	// notify that node was contacted successfully, with no closer nodes
	state = sm.Advance(ctx, &EventProbeMessageResponse[key.Key8, kadtest.StrAddr]{
		NodeInfo: unaddressedNodeInfo[key.Key8, kadtest.StrAddr]{
			NodeID: st.NodeID,
		},
		Response: kadtest.NewResponse("resp", []kad.NodeInfo[key.Key8, kadtest.StrAddr]{
			kadtest.NewInfo(kadtest.NewID(key.Key8(4)), []kadtest.StrAddr{"addr_4"}),
			kadtest.NewInfo(kadtest.NewID(key.Key8(6)), []kadtest.StrAddr{"addr_6"}),
		}),
	})

	// node remains in routing table
	_, found := rt.GetNode(key.Key8(4))
	require.True(t, found)

	// state machine now idle
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by another revisit interval
	clk.Add(cfg.CheckInterval)

	// the probe state machine should attempt to contact node again, now it is time
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the connectivity check should be for the right node
	require.True(t, key.Equal(key.Key8(4), st.NodeID.Key()))

	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeWaitingWithCapacity{}, state)
}

func TestProbeMessageFailure(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute

	// Set concurrency to allow more than one check to run
	cfg.Concurrency = 2

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// after adding first node the probe should be idle since the
	// connectivity check will be scheduled for the future
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by one revisit interval
	clk.Add(cfg.CheckInterval)
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the probe state machine should attempt to contact the next node
	st := state.(*StateProbeConnectivityCheck[key.Key8])

	// notify that node was contacted successfully, with no closer nodes
	state = sm.Advance(ctx, &EventProbeMessageFailure[key.Key8, kadtest.StrAddr]{
		NodeInfo: unaddressedNodeInfo[key.Key8, kadtest.StrAddr]{
			NodeID: st.NodeID,
		},
	})

	// state machine announces node failure
	require.IsType(t, &StateProbeNodeFailure[key.Key8]{}, state)
	stf := state.(*StateProbeNodeFailure[key.Key8])

	// the failure should be for the right node
	require.True(t, key.Equal(key.Key8(4), stf.NodeID.Key()))

	// node has been removed from routing table
	_, found := rt.GetNode(key.Key8(4))
	require.False(t, found)

	// advance time by another revisit interval
	clk.Add(cfg.CheckInterval)

	// state machine still idle since node was removed
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeIdle{}, state)
}

func TestProbeNotifyConnectivity(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute
	cfg.Concurrency = 2

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))
	rt.AddNode(kadtest.NewID(key.Key8(3)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// after adding first node the probe should be idle since the
	// connectivity check will be scheduled for the future (t0+10)
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})

	// not time for a check yet
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by less than the revisit interval
	// time is now (t0+2)
	clk.Add(2 * time.Minute)

	// add a second node, which will be second in the probe list since it's
	// time of next check will be later (t0+2+10=t0+12)
	state = sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(3)),
	})

	// still not time for a check
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time past the first node's check time but before the second node's
	// time is now (t0+2+9=t0+11)
	clk.Add(9 * time.Minute)

	// notify that the node with key 4 was connected to successfully by another process
	// this will delay the time for the next check to t0+11+10=to+21
	state = sm.Advance(ctx, &EventProbeNotifyConnectivity[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})

	// still not time for a check
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time past second node's check time
	// time is now (t0+2+9+4=t0+15)
	clk.Add(4 * time.Minute)

	// Poll the state machine, it should now attempt to contact a node
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)

	// the connectivity check should be for the right node, which is the one
	// that did not get a connectivity notification
	st := state.(*StateProbeConnectivityCheck[key.Key8])
	require.True(t, key.Equal(key.Key8(3), st.NodeID.Key()))

	// Poll the state machine, it should now waiting for a response but still have capacity
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeWaitingWithCapacity{}, state)
}

func TestProbeTimeout(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	cfg := DefaultProbeConfig()
	cfg.Clock = clk
	cfg.CheckInterval = 10 * time.Minute
	cfg.Timeout = 3 * time.Minute
	cfg.Concurrency = 1 // one probe at a time, timeouts will be used to free capacity if there are more requests

	rt := simplert.New[key.Key8, kad.NodeID[key.Key8]](kadtest.NewID(key.Key8(128)), 5)
	rt.AddNode(kadtest.NewID(key.Key8(4)))
	rt.AddNode(kadtest.NewID(key.Key8(3)))

	sm, err := NewProbe[key.Key8, kadtest.StrAddr](rt, cfg)
	require.NoError(t, err)

	// add a node
	state := sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(4)),
	})

	// not time for a check yet
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time a little
	clk.Add(time.Minute)

	// add another node
	state = sm.Advance(ctx, &EventProbeAdd[key.Key8]{
		NodeID: kadtest.NewID(key.Key8(3)),
	})

	// not time for a check yet
	require.IsType(t, &StateProbeIdle{}, state)

	// advance time by check interval
	clk.Add(cfg.CheckInterval)

	// poll state machine
	state = sm.Advance(ctx, &EventProbePoll{})

	// the connectivity check should start
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)
	stm := state.(*StateProbeConnectivityCheck[key.Key8])
	require.True(t, key.Equal(key.Key8(4), stm.NodeID.Key()))

	// Poll the state machine, it should now waiting for a response with no capacity
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeWaitingAtCapacity{}, state)

	// advance time past the timeout
	clk.Add(cfg.Timeout)

	// state machine announces node failure
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeNodeFailure[key.Key8]{}, state)
	stf := state.(*StateProbeNodeFailure[key.Key8])

	// the failure should be for the right node
	require.True(t, key.Equal(key.Key8(4), stf.NodeID.Key()))

	// node has been removed from routing table
	_, found := rt.GetNode(key.Key8(4))
	require.False(t, found)

	// state machine starts check for next node now there is capacity
	state = sm.Advance(ctx, &EventProbePoll{})
	require.IsType(t, &StateProbeConnectivityCheck[key.Key8]{}, state)
	stm = state.(*StateProbeConnectivityCheck[key.Key8])
	require.True(t, key.Equal(key.Key8(3), stm.NodeID.Key()))
}
