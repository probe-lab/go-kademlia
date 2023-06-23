package litesimulator

import (
	"context"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p-kad-dht/events/planner"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler"
	"github.com/libp2p/go-libp2p-kad-dht/events/simulator"
	"github.com/libp2p/go-libp2p-kad-dht/util"
)

type LiteSimulator struct {
	clk        *clock.Mock
	schedulers []scheduler.AwareScheduler // replace with custom linked list
}

var _ simulator.Simulator = (*LiteSimulator)(nil)

func NewLiteSimulator(clk *clock.Mock) *LiteSimulator {
	return &LiteSimulator{
		clk:        clk,
		schedulers: make([]scheduler.AwareScheduler, 0),
	}
}

func (s *LiteSimulator) AddPeer(sched scheduler.AwareScheduler) {
	s.schedulers = append(s.schedulers, sched)
}

func (s *LiteSimulator) RemovePeer(sched scheduler.AwareScheduler) {
	for i, sch := range s.schedulers {
		if sch == sched {
			s.schedulers = append(s.schedulers[:i], s.schedulers[i+1:]...)
		}
	}
}

func (s *LiteSimulator) Run(ctx context.Context) {
	ctx, span := util.StartSpan(ctx, "SimpleDispatcher.DispatchLoop")
	defer span.End()

	// get the next action time for each peer
	nextActions := make([]time.Time, len(s.schedulers))
	for i, sched := range s.schedulers {
		nextActions[i] = sched.NextActionTime(ctx)
	}

	for len(nextActions) > 0 {
		// find the time of the next action to be run
		minTime := planner.MaxTime
		for _, t := range nextActions {
			if t.Before(minTime) {
				minTime = t
			}
		}

		if minTime == planner.MaxTime {
			// no more actions to run
			break
		}

		upNext := make([]int, 0)
		for id, t := range nextActions {
			if t == minTime {
				upNext = append(upNext, id)
			}
		}

		if minTime.After(s.clk.Now()) {
			// "wait" minTime for the next action
			s.clk.Set(minTime) // slow to execute (because of the mutex?)
		}

		for len(upNext) > 0 {
			ongoing := make([]int, len(upNext))
			copy(ongoing, upNext)

			upNext = make([]int, 0)
			for _, id := range ongoing {
				// run one action for this peer
				s.schedulers[id].RunOne(ctx)
			}
			for id, s := range s.schedulers {
				t := s.NextActionTime(ctx)
				if t == minTime {
					upNext = append(upNext, id)
				} else {
					nextActions[id] = t
				}
			}
		}
	}
}
