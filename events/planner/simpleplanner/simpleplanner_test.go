package simpleplanner

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p-kad-dht/events/action"
	ta "github.com/libp2p/go-libp2p-kad-dht/events/action/testaction"
	"github.com/libp2p/go-libp2p-kad-dht/events/planner"
	"github.com/stretchr/testify/require"
)

func TestSimplePlanner(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	p := NewSimplePlanner(clk)

	nActions := 10
	actions := make([]action.Action, nActions)
	for i := 0; i < nActions; i++ {
		actions[i] = ta.IntAction(i)
	}

	a0 := p.ScheduleAction(ctx, clk.Now().Add(time.Millisecond), actions[0])
	require.Empty(t, p.PopOverdueActions(ctx))

	clk.Add(time.Millisecond)
	require.Equal(t, actions[:1], p.PopOverdueActions(ctx))
	require.Empty(t, p.PopOverdueActions(ctx))

	p.ScheduleAction(ctx, clk.Now().Add(2*time.Minute), actions[1])
	p.ScheduleAction(ctx, clk.Now().Add(2*time.Second), actions[2])
	p.ScheduleAction(ctx, clk.Now().Add(time.Minute), actions[3])
	p.ScheduleAction(ctx, clk.Now().Add(time.Hour), actions[4])
	require.Empty(t, p.PopOverdueActions(ctx))

	clk.Add(2 * time.Second)
	require.Equal(t, actions[2:3], p.PopOverdueActions(ctx))

	clk.Add(2 * time.Minute)
	require.Equal(t, []action.Action{actions[3], actions[1]}, p.PopOverdueActions(ctx))

	p.ScheduleAction(ctx, clk.Now().Add(time.Second), actions[5])
	clk.Add(time.Second)
	require.Equal(t, actions[5:6], p.PopOverdueActions(ctx))

	clk.Add(time.Hour)
	require.Equal(t, actions[4:5], p.PopOverdueActions(ctx))

	p.RemoveAction(ctx, a0)

	a6 := p.ScheduleAction(ctx, clk.Now().Add(time.Second), actions[6])      // 3
	p.ScheduleAction(ctx, clk.Now().Add(time.Microsecond), actions[7])       // 1
	a8 := p.ScheduleAction(ctx, clk.Now().Add(time.Hour), actions[8])        // 4
	a9 := p.ScheduleAction(ctx, clk.Now().Add(time.Millisecond), actions[9]) // 2

	p.RemoveAction(ctx, a9)
	p.RemoveAction(ctx, a0)
	clk.Add(time.Second)

	p.RemoveAction(ctx, a6)
	require.Equal(t, actions[7:8], p.PopOverdueActions(ctx))

	p.RemoveAction(ctx, a8)
	require.Empty(t, p.PopOverdueActions(ctx))
}

type otherPlannedAction int

func (a otherPlannedAction) Action() action.Action {
	return ta.IntAction(int(a))
}

func (a otherPlannedAction) Time() time.Time {
	return time.Time{}
}

func TestSimplePlannedAction(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	p := NewSimplePlanner(clk)

	a0 := p.ScheduleAction(ctx, clk.Now().Add(time.Millisecond), ta.IntAction(0))
	require.Equal(t, ta.IntAction(0), a0.Action())
	require.Equal(t, clk.Now().Add(time.Millisecond), a0.Time())

	a1 := otherPlannedAction(1)
	p.RemoveAction(ctx, a1)

	clk.Add(time.Millisecond)

	overdueActions := p.PopOverdueActions(ctx)
	require.Equal(t, 1, len(overdueActions))
	require.Equal(t, a0.Action(), overdueActions[0])
	require.Nil(t, p.PopOverdueActions(ctx))
}

func TestNextActionTime(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()
	p := NewSimplePlanner(clk)

	clk.Set(time.Unix(0, 0))
	actions := make([]action.Action, 3)
	for i := 0; i < len(actions); i++ {
		actions[i] = ta.IntAction(i)
	}

	ti := p.NextActionTime(ctx)
	require.Equal(t, planner.MaxTime, ti)

	t0 := clk.Now().Add(time.Second)
	p.ScheduleAction(ctx, t0, actions[0])
	ti = p.NextActionTime(ctx)
	require.Equal(t, t0, ti)

	t1 := clk.Now().Add(time.Hour)
	p.ScheduleAction(ctx, t1, actions[1])
	ti = p.NextActionTime(ctx)
	require.Equal(t, t0, ti)

	t2 := clk.Now().Add(time.Millisecond)
	p.ScheduleAction(ctx, t2, actions[2])
	ti = p.NextActionTime(ctx)
	require.Equal(t, t2, ti)

	require.Equal(t, 0, len(p.PopOverdueActions(ctx)))

	clk.Add(time.Millisecond)
	ti = p.NextActionTime(ctx)
	require.Equal(t, t2, ti)

	require.Equal(t, 1, len(p.PopOverdueActions(ctx)))
	ti = p.NextActionTime(ctx)
	require.Equal(t, t0, ti)

	clk.Add(time.Hour)
	require.Equal(t, 2, len(p.PopOverdueActions(ctx)))
	ti = p.NextActionTime(ctx)
	require.Equal(t, planner.MaxTime, ti)
}
