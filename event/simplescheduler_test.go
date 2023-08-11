package event

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/require"
)

func TestSimpleScheduler(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	sched := NewSimpleScheduler(clk)

	require.Equal(t, clk.Now(), sched.Clock().Now())

	nActions := 10
	actions := make([]*FuncAction, nActions)

	for i := 0; i < nActions; i++ {
		actions[i] = NewFuncAction(i)
	}

	sched.EnqueueAction(ctx, actions[0])
	require.False(t, actions[0].Ran)
	sched.RunOne(ctx)
	require.True(t, actions[0].Ran)

	ScheduleActionIn(ctx, sched, time.Second, actions[1])
	require.False(t, actions[1].Ran)
	sched.EnqueueAction(ctx, actions[2])
	clk.Add(2 * time.Second)

	sched.RunOne(ctx)
	require.True(t, actions[2].Ran)
	require.False(t, actions[1].Ran)
	sched.RunOne(ctx)
	require.True(t, actions[1].Ran)
	sched.RunOne(ctx)

	ScheduleActionIn(ctx, sched, -1*time.Second, actions[3])
	require.False(t, actions[3].Ran)
	sched.RunOne(ctx)
	require.True(t, actions[3].Ran)

	sched.ScheduleAction(ctx, clk.Now().Add(-1*time.Nanosecond), actions[4])
	require.False(t, actions[4].Ran)
	sched.RunOne(ctx)
	require.True(t, actions[4].Ran)

	sched.ScheduleAction(ctx, clk.Now().Add(time.Second), actions[5])
	sched.RunOne(ctx)
	require.False(t, actions[5].Ran)
	clk.Add(time.Second)
	require.Equal(t, clk.Now(), sched.NextActionTime(ctx))
	sched.RunOne(ctx)
	require.True(t, actions[5].Ran)

	t6 := clk.Now().Add(time.Second)
	a6 := sched.ScheduleAction(ctx, t6, actions[6])
	require.Equal(t, t6, sched.NextActionTime(ctx))
	sched.RemovePlannedAction(ctx, a6)
	clk.Add(time.Second)
	sched.RunOne(ctx)
	require.False(t, actions[6].Ran)
	// empty queue
	require.Equal(t, MaxTime, sched.NextActionTime(ctx))
}
