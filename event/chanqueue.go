package event

import (
	"context"

	"github.com/plprobelab/go-libdht/util"
)

// ChanQueue is a trivial queue implementation using a channel
type ChanQueue struct {
	queue chan Action
}

var _ EventQueueWithEmpty = (*ChanQueue)(nil)

// NewChanQueue creates a new queue
func NewChanQueue(capacity int) *ChanQueue {
	return &ChanQueue{
		queue: make(chan Action, capacity),
	}
}

// Enqueue adds an element to the queue
func (q *ChanQueue) Enqueue(ctx context.Context, e Action) {
	_, span := util.StartSpan(ctx, "ChanQueue.Enqueue")
	defer span.End()
	q.queue <- e
}

// Dequeue reads the next element from the queue, note that this operation is blocking
func (q *ChanQueue) Dequeue(ctx context.Context) Action {
	_, span := util.StartSpan(ctx, "ChanQueue.Dequeue")
	defer span.End()

	if q.Empty() {
		span.AddEvent("empty queue")
		return nil
	}

	return <-q.queue
}

// Empty returns true if the queue is empty
func (q *ChanQueue) Empty() bool {
	return len(q.queue) == 0
}

func (q *ChanQueue) Size() uint {
	return uint(len(q.queue))
}

func (q *ChanQueue) Close() {
	close(q.queue)
}
