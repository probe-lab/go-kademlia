package main

import (
	"context"
)

// A State is the result produced by running a Task.
// If the Task failed to complete its work the Err method must return a non-nil error.
type State interface {
	// String returns a short description of the State.
	// commented for rapid iteration on states at the moment
	// String() string
}

// Stopped is a sentinel state that indicates that the task has stopped work and will not resume it.
var Stopped stopped

type stopped struct{}

var _ State = stopped{}

func (stopped) String() string {
	return "stopped"
}

// Pending is a sentinel state that indicates that work is still ongoing and has not been completed.
var Pending pstate

type pstate struct{}

var _ State = pstate{}

func (pstate) String() string {
	return "pending"
}

// A Task represents a state machine that is performing work.
type Task[T State] interface {
	// Advance signals the task to advance its work to the next state.
	Advance(context.Context) T

	// Cancel signals the Task to stop work.
	// After a call to Cancel, Advance must always return Stopped
	Cancel(context.Context)
}
