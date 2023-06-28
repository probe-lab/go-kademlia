package basicaction

import (
	"context"

	"github.com/plprobelab/go-kademlia/events/action"
)

// BasicAction is a basic implementation of the Action interface
type BasicAction func(context.Context)

var _ action.Action = (*BasicAction)(nil)

// Run executes the action
func (a BasicAction) Run(ctx context.Context) {
	a(ctx)
}
