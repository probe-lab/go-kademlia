package queue

import (
	"context"

	"github.com/libp2p/go-libp2p-kad-dht/events/action"
)

type EventQueue interface {
	Enqueue(context.Context, action.Action)
	Dequeue(context.Context) action.Action

	Size() uint
	Close()
}

type EventQueueEnqueueMany interface {
	EventQueue
	EnqueueMany(context.Context, []action.Action)
}

func EnqueueMany(ctx context.Context, q EventQueue, actions []action.Action) {
	switch queue := q.(type) {
	case EventQueueEnqueueMany:
		queue.EnqueueMany(ctx, actions)
	default:
		for _, a := range actions {
			q.Enqueue(ctx, a)
		}
	}
}

type EventQueueDequeueMany interface {
	DequeueMany(context.Context, int) []action.Action
}

func DequeueMany(ctx context.Context, q EventQueue, n int) []action.Action {
	switch queue := q.(type) {
	case EventQueueDequeueMany:
		return queue.DequeueMany(ctx, n)
	default:
		actions := make([]action.Action, 0, n)
		for i := 0; i < n; i++ {
			if a := q.Dequeue(ctx); a != nil {
				actions = append(actions, a)
			} else {
				break
			}
		}
		return actions
	}
}

type EventQueueDequeueAll interface {
	DequeueAll(context.Context) []action.Action
}

func DequeueAll(ctx context.Context, q EventQueue) []action.Action {
	switch queue := q.(type) {
	case EventQueueDequeueAll:
		return queue.DequeueAll(ctx)
	default:
		actions := make([]action.Action, 0, q.Size())
		for a := q.Dequeue(ctx); a != nil; a = q.Dequeue(ctx) {
			actions = append(actions, a)
		}
		return actions
	}
}

type EventQueueWithEmpty interface {
	EventQueue
	Empty() bool
}

func Empty(q EventQueue) bool {
	switch queue := q.(type) {
	case EventQueueWithEmpty:
		return queue.Empty()
	default:
		return q.Size() == 0
	}
}
