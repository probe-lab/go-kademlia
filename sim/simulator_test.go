package sim

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/plprobelab/go-kademlia/event"
	"github.com/stretchr/testify/require"
)

func TestLiteSimulator(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewMock()

	nNodes := 7
	scheds := make([]event.AwareScheduler, nNodes)

	for i := 0; i < nNodes; i++ {
		scheds[i] = event.NewSimpleScheduler(clk)
	}

	sim := NewLiteSimulator(clk)
	AddSchedulers(sim, scheds...)

	RemoveSchedulers(sim, scheds[0], scheds[3], scheds[6])
	require.Equal(t, []event.AwareScheduler{
		scheds[1], scheds[2],
		scheds[4], scheds[5],
	}, sim.schedulers)

	sim.Run(ctx)

	order := []int{}
	// enqueue an action on 4 (that will be executed after action enqueue on 2)
	// this action will enqueue a new action on 1 (that will be executed later)
	scheds[4].EnqueueAction(ctx, event.BasicAction(func(context.Context) {
		order = append(order, 1)
		scheds[1].EnqueueAction(ctx, event.BasicAction(func(context.Context) {
			order = append(order, 2)
		}))
	}))
	scheds[2].EnqueueAction(ctx, event.BasicAction(func(context.Context) {
		order = append(order, 0)
	}))

	sim.Run(ctx)

	require.Len(t, order, 3)
	for i, e := range order {
		require.Equal(t, i, e)
	}

	order = []int{}
	event.ScheduleActionIn(ctx, scheds[1], time.Minute,
		event.BasicAction(func(context.Context) {
			order = append(order, 3)
		}))
	event.ScheduleActionIn(ctx, scheds[2], time.Second,
		event.BasicAction(func(context.Context) {
			order = append(order, 1)
			scheds[1].EnqueueAction(ctx, event.BasicAction(func(context.Context) {
				order = append(order, 2)
			}))
		}))
	scheds[4].EnqueueAction(ctx, event.BasicAction(func(context.Context) {
		order = append(order, 0)
	}))

	sim.Run(ctx)

	require.Len(t, order, 4)
	for i, e := range order {
		require.Equal(t, i, e)
	}
}
