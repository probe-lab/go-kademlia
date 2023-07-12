// Package task provides types and functions for performing and tracking the progress of scheduled work.
package task

import (
	"context"
	"sync/atomic"
)

// A State is the result produced by running a Task.
// If the Task failed to complete its work the Err method must return a non-nil error.
type State interface {
	// String returns a short description of the State.
	// commented for rapid iteration on states at the moment
	// String() string
}

// Cancelled is a sentinel state that indicates that work has been cancelled or aborted before completion.
var Cancelled cstate

type cstate struct{}

var _ State = cstate{}

func (cstate) String() string {
	return "cancelled"
}

// Pending is a sentinel state that indicates that work is still ongoing and has not been completed.
var Pending pstate

type pstate struct{}

var _ State = pstate{}

func (pstate) String() string {
	return "pending"
}

// A Runnable is something that starts work when Run is called.
type Runnable interface {
	// Run signals the Runnable to begin its work. The result of the work must be sent
	// as a State on the provided channel. Run must not send more than one state
	// to the channel but it may close the channel with or without writing a state.
	// Closing the channel without writing a state indicates that the work has
	// been cancelled by the Runnable. A Task reading the closed channel will
	// indicated this by returning a Cancelled state.
	Run(context.Context, chan<- State)
}

// A RunnableFunc adapts a function as a Runnable
type RunnableFunc func(context.Context, chan<- State)

func (f RunnableFunc) Run(ctx context.Context, ch chan<- State) {
	f(ctx, ch)
}

type Task interface {
	// Advance signals the task to advance its work to the next state.
	Advance(context.Context) State

	// Cancel signals the Task to stop work.
	// After a call to Cancel, Advance must always return Cancelled
	Cancel(context.Context)
}

// An AsyncTask is a task that performs work asynchronously.
type AsyncTask struct {
	ch        <-chan State
	cancel    func()
	cancelled atomic.Bool
}

var _ Task = (*AsyncTask)(nil)

// Async starts r in a gorutine by calling r.Run and returns an AsyncTask to track completion of r's work.
func Async(ctx context.Context, r Runnable) *AsyncTask {
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan State)
	go r.Run(ctx, ch)
	return &AsyncTask{
		ch:     ch,
		cancel: cancel,
	}
}

func (t *AsyncTask) Advance(ctx context.Context) State {
	if t.cancelled.Load() {
		return Cancelled
	}
	select {
	case st, ok := <-t.ch:
		t.Cancel(ctx)
		if st == nil && !ok {
			return Cancelled
		}
		return st
	default:
		return Pending
	}
}

func (t *AsyncTask) Cancel(context.Context) {
	t.cancelled.Store(true)
	t.cancel()
}

// A StaticTask is a task that performs no work but carries a state. When Done is called it returns the state unless
// Cancel has been called previously, in which case it will return the Cancelled state.
type StaticTask struct {
	state     State
	cancelled atomic.Bool
}

var _ Task = (*StaticTask)(nil)

func Static(state State) *StaticTask {
	return &StaticTask{
		state: state,
	}
}

func (t *StaticTask) Advance(context.Context) State {
	if t.cancelled.Load() {
		return Cancelled
	}
	return t.state
}

func (t *StaticTask) Cancel(context.Context) {
	t.cancelled.Store(true)
}
