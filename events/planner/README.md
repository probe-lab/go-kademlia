# Action Planner

An `ActionPlanner` records `Action`s that needs to be run at a specific time in the future. It is used to handle timeouts in a multi thread application run by a single worker.

```go
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
```

Just like a `Queue` an `ActionPlanner` used in a multi thread context must be thread safe. It is fine to use a non thread safe `ActionPlanner` if the caller is also single threaded, e.g in testing or simulations.

## `PlannedAction`

A `PlannedAction` is basically a pointer to an `Action` sitting in an `ActionPlanner`. It serves to remove `Action`s from an `ActionPlanner` (it is necessary because `Action` is not `Comparable`).

```go
// PlannedAction is an interface for actions that are scheduled to run at a
// specific time.
type PlannedAction interface {
	// Time returns the time at which the action is scheduled to run
	Time() time.Time
	// Action returns the action that is scheduled to run
	Action() action.Action
}
```