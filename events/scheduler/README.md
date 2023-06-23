# Scheduler

A `Scheduler` is an interface wrapping a `Queue` and an `ActionPlanner`. It is the main abstraction for the single worker multi thread application. It exposes the same abstraction as a `Queue` and an `ActionPlanner` combined.

```go
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
```