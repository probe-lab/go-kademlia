package scheduler

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht/events/action"
	"github.com/libp2p/go-libp2p-kad-dht/events/planner"
)

// Scheduler is an interface for scheduling actions to run as soon as possible
// or at a specific time
type Scheduler interface {
	// Now returns the time of the scheduler's clock
	Now() time.Time

	// EnqueueAction enqueues an action to run as soon as possible
	EnqueueAction(context.Context, action.Action)
	// ScheduleAction schedules an action to run at a specific time
	ScheduleAction(context.Context, time.Time, action.Action) planner.PlannedAction
	// RemovePlannedAction removes an action from the scheduler planned actions
	// (not from the queue), does nothing if the action is not in the planner
	RemovePlannedAction(context.Context, planner.PlannedAction)

	// RunOne runs one action from the scheduler's queue, returning true if an
	// action was run, false if the queue was empty
	RunOne(context.Context) bool
}

// ScheduleActionIn schedules an action to run after a delay
func ScheduleActionIn(ctx context.Context, s Scheduler, d time.Duration, a action.Action) planner.PlannedAction {
	if d <= 0 {
		s.EnqueueAction(ctx, a)
		return nil
	} else {
		return s.ScheduleAction(ctx, s.Now().Add(d), a)
	}
}

// RunManyScheduler is a scheduler that can run multiple actions at once
type RunManyScheduler interface {
	Scheduler

	// RunMany runs n actions on the scheduler, returning true if all actions
	// were run, or false if there were less than n actions to run
	RunMany(context.Context, int) bool
}

// RunMany runs n actions on the scheduler, returning true if all actions were
// run, or false if there were less than n actions to run
func RunMany(ctx context.Context, s Scheduler, n int) bool {
	switch s := s.(type) {
	case RunManyScheduler:
		return s.RunMany(ctx, n)
	default:
		for i := 0; i < n; i++ {
			if !s.RunOne(ctx) {
				return false
			}
		}
		return true
	}
}

// RunAllScheduler is a scheduler that can run all actions in its queue
type RunAllScheduler interface {
	Scheduler

	// RunAll runs all actions in the scheduler's queue
	RunAll(context.Context)
}

// RunAll runs all actions in the scheduler's queue and overdue actions from
// the planner
func RunAll(ctx context.Context, s Scheduler) {
	switch s := s.(type) {
	case RunAllScheduler:
		s.RunAll(ctx)
	default:
		for s.RunOne(ctx) {
		}
	}
}

// AwareScheduler is a scheduler that can return the time of the next scheduled
// action.
type AwareScheduler interface {
	Scheduler

	// NextActionTime returns the time of the next action in the scheduler's
	// queue or util.MaxTime if the queue is empty
	NextActionTime(context.Context) time.Time
}
