package planner

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-kad-dht/events/action"
)

// MaxTime is the maximum time.Time value
var MaxTime = time.Unix(1<<63-62135596801, 999999999)

// PlannedAction is an interface for actions that are scheduled to run at a
// specific time.
type PlannedAction interface {
	// Time returns the time at which the action is scheduled to run
	Time() time.Time
	// Action returns the action that is scheduled to run
	Action() action.Action
}

// ActionPlanner is an interface for scheduling actions at a specific time.
type ActionPlanner interface {
	// ScheduleAction schedules an action to run at a specific time
	ScheduleAction(context.Context, time.Time, action.Action) PlannedAction
	// RemoveAction removes an action from the planner
	RemoveAction(context.Context, PlannedAction)

	// PopOverdueActions returns all actions that are overdue and removes them
	// from the planner
	PopOverdueActions(context.Context) []action.Action
}

// MultiActionPlanner is an interface for scheduling multiple actions at
// specific times.
type MultiActionPlanner interface {
	ActionPlanner

	// ScheduleActions schedules multiple actions at specific times
	ScheduleActions(context.Context, []time.Time, []action.Action) []PlannedAction
	// RemoveActions removes multiple actions from the planner
	RemoveActions(context.Context, []PlannedAction)
}

// ScheduleActions schedules multiple actions at specific times using a planner.
func ScheduleActions(ctx context.Context, p ActionPlanner,
	times []time.Time, actions []action.Action) []PlannedAction {

	if len(times) != len(actions) {
		return nil
	}

	switch p := p.(type) {
	case MultiActionPlanner:
		return p.ScheduleActions(ctx, times, actions)
	default:
		res := make([]PlannedAction, len(times))
		for i, d := range times {
			p.ScheduleAction(ctx, d, actions[i])
		}
		return res
	}
}

// RemoveActions removes multiple actions from the planner.
func RemoveActions(ctx context.Context, p ActionPlanner, actions []PlannedAction) {
	switch p := p.(type) {
	case MultiActionPlanner:
		p.RemoveActions(ctx, actions)
	default:
		for _, a := range actions {
			p.RemoveAction(ctx, a)
		}
	}
}

// AwareActionPlanner is an interface for scheduling actions at a specific time
// and knowing when the next action will be scheduled.
type AwareActionPlanner interface {
	ActionPlanner

	// NextActionTime returns the time of the next action that will be
	// scheduled. If there are no actions scheduled, it returns MaxTime.
	NextActionTime(context.Context) time.Time
}
