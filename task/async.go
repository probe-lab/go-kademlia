package task

import (
	"context"
	"sync/atomic"
)

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
		return Stopped
	}
	select {
	case st, ok := <-t.ch:
		t.Cancel(ctx)
		if st == nil && !ok {
			return Stopped
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

// A Runnable is something that starts work when Run is called.
type Runnable interface {
	// Run signals the Runnable to begin its work. The result of the work must be sent
	// as a State on the provided channel. Run must not send more than one state
	// to the channel but it may close the channel with or without writing a state.
	// Closing the channel without writing a state indicates that the work has
	// been cancelled by the Runnable. A Task reading the closed channel will
	// indicated this by returning a Stopped state.
	Run(context.Context, chan<- State)
}

// A RunnableFunc adapts a function as a Runnable
type RunnableFunc func(context.Context, chan<- State)

func (f RunnableFunc) Run(ctx context.Context, ch chan<- State) {
	f(ctx, ch)
}
