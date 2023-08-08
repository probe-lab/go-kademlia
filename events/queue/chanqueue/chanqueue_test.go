package chanqueue

import (
	"context"
	"testing"

	"github.com/plprobelab/go-kademlia/events/action"
	ta "github.com/plprobelab/go-kademlia/events/action/testaction"
	"github.com/stretchr/testify/require"
)

func TestChanQueue(t *testing.T) {
	ctx := context.Background()
	nEvents := 10
	events := make([]action.Action, nEvents)
	for i := 0; i < nEvents; i++ {
		events[i] = ta.IntAction(i)
	}

	q := NewChanQueue(nEvents)
	if q.Size() != 0 {
		t.Errorf("Expected size 0, got %d", q.Size())
	}
	require.True(t, q.Empty())

	q.Enqueue(ctx, events[0])
	if q.Size() != 1 {
		t.Errorf("Expected size 1, got %d", q.Size())
	}
	require.False(t, q.Empty())

	q.Enqueue(ctx, events[1])
	if q.Size() != 2 {
		t.Errorf("Expected size 2, got %d", q.Size())
	}
	require.False(t, q.Empty())

	if !q.Empty() {
		e := q.Dequeue(ctx)
		require.Equal(t, e, events[0])
		if q.Size() != 1 {
			t.Errorf("Expected size 1, got %d", q.Size())
		}
		require.False(t, q.Empty())
	}

	if !q.Empty() {
		e := q.Dequeue(ctx)
		require.Equal(t, e, events[1])
		if q.Size() != 0 {
			t.Errorf("Expected size 0, got %d", q.Size())
		}
		require.True(t, q.Empty())
	}

	require.Nil(t, q.Dequeue(ctx))

	q.Close()
}
