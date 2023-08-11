package event

import (
	"context"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
)

type SimplePlanner struct {
	Clock clock.Clock

	NextAction *simpleTimedAction
	lock       sync.Mutex
}

var _ AwareActionPlanner = (*SimplePlanner)(nil)

type simpleTimedAction struct {
	action Action
	time   time.Time
	next   *simpleTimedAction
}

var _ PlannedAction = (*simpleTimedAction)(nil)

func (a *simpleTimedAction) Time() time.Time {
	return a.time
}

func (a *simpleTimedAction) Action() Action {
	return a.action
}

func NewSimplePlanner(clk clock.Clock) *SimplePlanner {
	return &SimplePlanner{
		Clock: clk,
	}
}

func (p *SimplePlanner) ScheduleAction(ctx context.Context, t time.Time, a Action) PlannedAction {
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

func (p *SimplePlanner) RemoveAction(ctx context.Context, pa PlannedAction) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	a, ok := pa.(*simpleTimedAction)
	if !ok {
		return false
	}

	curr := p.NextAction
	if curr == nil {
		return false
	}

	if curr == a {
		p.NextAction = curr.next
		return true
	}
	for curr.next != nil {
		if curr.next == a {
			curr.next = curr.next.next
			return true
		}
		curr = curr.next
	}
	return false
}

func (p *SimplePlanner) PopOverdueActions(ctx context.Context) []Action {
	p.lock.Lock()
	defer p.lock.Unlock()

	var overdue []Action
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
		return MaxTime
	}
	return p.NextAction.time
}
