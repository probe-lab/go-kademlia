package event

import (
	"context"
)

// A BasicAction is the default Action used for event scheduling in the Kademlia implementation.
type BasicAction func(context.Context)

var _ Action = (*BasicAction)(nil)

// Run executes the action
func (a BasicAction) Run(ctx context.Context) {
	a(ctx)
}
