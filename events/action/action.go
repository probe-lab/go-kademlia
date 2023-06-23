package action

import "context"

// Action is an interface for an action that can be run. It is used as unit by
// the scheduler (event queue + planner)
type Action interface {
	Run(context.Context)
}
