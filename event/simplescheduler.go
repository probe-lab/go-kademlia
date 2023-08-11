package event

import (
	"context"
	"time"

	"github.com/benbjohnson/clock"
)

const DefaultChanqueueCapacity = 1024

// SimpleScheduler is a simple implementation of the Scheduler interface. It
// uses a simple planner and a channel-based queue.
type SimpleScheduler struct {
	clk clock.Clock

	queue   EventQueue
	planner AwareActionPlanner
}

var _ AwareScheduler = (*SimpleScheduler)(nil)

// NewSimpleScheduler creates a new SimpleScheduler.
func NewSimpleScheduler(clk clock.Clock) *SimpleScheduler {
	return &SimpleScheduler{
		clk: clk,

		queue:   NewChanQueue(DefaultChanqueueCapacity),
		planner: NewSimplePlanner(clk),
	}
}

// Now returns the scheduler's current time.
func (s *SimpleScheduler) Clock() clock.Clock {
	return s.clk
}

// EnqueueAction enqueues an action to be run as soon as possible.
func (s *SimpleScheduler) EnqueueAction(ctx context.Context, a Action) {
	s.queue.Enqueue(ctx, a)
}

// ScheduleAction schedules an action to run at a specific time.
func (s *SimpleScheduler) ScheduleAction(ctx context.Context, t time.Time,
	a Action,
) PlannedAction {
	if s.clk.Now().After(t) {
		s.EnqueueAction(ctx, a)
		return nil
	}
	return s.planner.ScheduleAction(ctx, t, a)
}

// RemovePlannedAction removes an action from the scheduler planned actions
// (not from the queue), does nothing if the action is not in the planner
func (s *SimpleScheduler) RemovePlannedAction(ctx context.Context, a PlannedAction) bool {
	return s.planner.RemoveAction(ctx, a)
}

// moveOverdueActions moves all overdue actions from the planner to the queue.
func (s *SimpleScheduler) moveOverdueActions(ctx context.Context) {
	overdue := s.planner.PopOverdueActions(ctx)

	EnqueueMany(ctx, s.queue, overdue)
}

// RunOne runs one action from the scheduler's queue, returning true if an
// action was run, false if the queue was empty.
func (s *SimpleScheduler) RunOne(ctx context.Context) bool {
	s.moveOverdueActions(ctx)

	if a := s.queue.Dequeue(ctx); a != nil {
		a.Run(ctx)
		return true
	}
	return false
}

// NextActionTime returns the time of the next action to run, or the current
// time if there are actions to be run in the queue, or util.MaxTime if there
// are no scheduled to run.
func (s *SimpleScheduler) NextActionTime(ctx context.Context) time.Time {
	s.moveOverdueActions(ctx)
	nextScheduled := s.planner.NextActionTime(ctx)

	if !Empty(s.queue) {
		return s.clk.Now()
	}
	return nextScheduled
}
