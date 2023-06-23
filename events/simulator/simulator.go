package simulator

import (
	"context"

	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler"
)

// Simulator is an interface for simulating a set of schedulers.
type Simulator interface {
	// AddPeer adds a scheduler to the simulator
	AddPeer(scheduler.AwareScheduler)
	// RemovePeer removes a scheduler from the simulator
	RemovePeer(scheduler.AwareScheduler)
	// Run runs the simulator until there are no more Actions to run
	Run(context.Context)
}

// AddPeers adds a set of schedulers to a simulator
func AddPeers(s Simulator, schedulers ...scheduler.AwareScheduler) {
	for _, sched := range schedulers {
		s.AddPeer(sched)
	}
}

// RemovePeers removes a set of schedulers from a simulator
func RemovePeers(s Simulator, schedulers ...scheduler.AwareScheduler) {
	for _, sched := range schedulers {
		s.RemovePeer(sched)
	}
}
