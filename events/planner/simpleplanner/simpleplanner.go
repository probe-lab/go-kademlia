package simpleplanner

import (
	"context"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p-kad-dht/events/action"
	"github.com/libp2p/go-libp2p-kad-dht/events/planner"
)

type SimplePlanner struct {
	Clock clock.Clock

	NextAction *simpleTimedAction
	lock       sync.Mutex
}

var _ planner.AwareActionPlanner = (*SimplePlanner)(nil)

type simpleTimedAction struct {
	action action.Action
	time   time.Time
	next   *simpleTimedAction
}

var _ planner.PlannedAction = (*simpleTimedAction)(nil)

func (a *simpleTimedAction) Time() time.Time {
	return a.time
}

func (a *simpleTimedAction) Action() action.Action {
	return a.action
}

func NewSimplePlanner(clk clock.Clock) *SimplePlanner {
	return &SimplePlanner{
		Clock: clk,
	}
}

func (p *SimplePlanner) ScheduleAction(ctx context.Context, t time.Time, a action.Action) planner.PlannedAction {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.NextAction == nil {
		p.NextAction = &simpleTimedAction{action: a, time: t}
		return p.NextAction
	}

	curr := p.NextAction
	if t.Before(curr.time) {
		p.NextAction = &simpleTimedAction{action: a, time: t, next: curr}
		return p.NextAction
	}
	for curr.next != nil && t.After(curr.next.time) {
		curr = curr.next
	}
	curr.next = &simpleTimedAction{action: a, time: t, next: curr.next}
	return curr.next
}

func (p *SimplePlanner) RemoveAction(ctx context.Context, pa planner.PlannedAction) {
	p.lock.Lock()
	defer p.lock.Unlock()

	a, ok := pa.(*simpleTimedAction)
	if !ok {
		return
	}

	curr := p.NextAction
	if curr == nil {
		return
	}

	if curr == a {
		p.NextAction = curr.next
		return
	}
	for curr.next != nil {
		if curr.next == a {
			curr.next = curr.next.next
			return
		}
		curr = curr.next
	}
}

func (p *SimplePlanner) PopOverdueActions(ctx context.Context) []action.Action {
	p.lock.Lock()
	defer p.lock.Unlock()

	var overdue []action.Action
	now := p.Clock.Now()
	curr := p.NextAction
	for curr != nil && (curr.time.Before(now) || curr.time == now) {
		overdue = append(overdue, curr.action)
		curr = curr.next
	}
	p.NextAction = curr
	return overdue
}

func (p *SimplePlanner) NextActionTime(context.Context) time.Time {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.NextAction == nil {
		return planner.MaxTime
	}
	return p.NextAction.time
}
