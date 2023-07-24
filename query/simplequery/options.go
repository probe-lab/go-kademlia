package simplequery

import (
	"context"
	"fmt"
	"time"

	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/routing"
)

// Config is a structure containing all the options that can be used when
// constructing a SimpleQuery.
type Config[K kad.Key[K]] struct {
	// ProtocolID is the protocol identifier used to send the request
	ProtocolID address.ProtocolID
	// NumberUsefulCloserPeers is the number of closer peers to look for in the
	// provided routing table when starting the request
	NumberUsefulCloserPeers int
	// Concurrency is the maximal number of simultaneous inflight requests for
	// this query
	Concurrency int

	// RequestTimeout is the timeout value for a single request
	RequestTimeout time.Duration
	// PeerstoreTTL is the TTL value for newly discovered peers in the peerstore
	PeerstoreTTL time.Duration

	// HandleResultFn is a function that is called when a response is received
	// for a request. It is used to determine whether the query should be
	// stopped and whether the peerlist should be updated.
	HandleResultsFunc HandleResultFn[K]
	// NotifyFailureFn is a function that is called when the query fails. It is
	// used to notify the user that the query failed.
	NotifyFailureFunc NotifyFailureFn

	// RoutingTable is the routing table used to find closer peers. It is
	// updated with newly discovered peers.
	RoutingTable routing.Table[K]
	// Endpoint is the message endpoint used to send requests
	Endpoint endpoint.Endpoint[K]
	// Scheduler is the scheduler used to schedule events for the single worker
	Scheduler scheduler.Scheduler
}

// Apply applies the SimpleQuery options to this Option
func (cfg *Config[K]) Apply(opts ...Option[K]) error {
	for i, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("SimpleQuery option %d failed: %s", i, err)
		}
	}
	if cfg.RoutingTable == nil {
		return fmt.Errorf("SimpleQuery option RoutingTable cannot be nil")
	}
	if cfg.Endpoint == nil {
		return fmt.Errorf("SimpleQuery option Endpoint cannot be nil")
	}
	if cfg.Scheduler == nil {
		return fmt.Errorf("SimpleQuery option Scheduler cannot be nil")
	}
	return nil
}

// Option type for SimpleQuery
type Option[K kad.Key[K]] func(*Config[K]) error

// DefaultConfig is the default options for SimpleQuery. This option is always
// prepended to the list of options passed to the SimpleQuery constructor.
// Note that most of the fields are left empty, and must be filled by the user.
func DefaultConfig[K kad.Key[K]](cfg *Config[K]) error {
	cfg.NumberUsefulCloserPeers = 20
	cfg.Concurrency = 10

	cfg.RequestTimeout = time.Second
	cfg.PeerstoreTTL = 30 * time.Minute

	cfg.HandleResultsFunc = func(ctx context.Context, id address.NodeID[K],
		resp message.MinKadResponseMessage[K],
	) (bool, []address.NodeID[K]) {
		ids := make([]address.NodeID[K], len(resp.CloserNodes()))
		for i, n := range resp.CloserNodes() {
			ids[i] = n.NodeID()
		}
		return false, ids
	}
	cfg.NotifyFailureFunc = func(context.Context) {}

	return nil
}

func WithProtocolID[K kad.Key[K]](pid address.ProtocolID) Option[K] {
	return func(cfg *Config[K]) error {
		cfg.ProtocolID = pid
		return nil
	}
}

func WithNumberUsefulCloserPeers[K kad.Key[K]](n int) Option[K] {
	return func(cfg *Config[K]) error {
		if n <= 0 {
			return fmt.Errorf("NumberUsefulCloserPeers must be positive")
		}
		cfg.NumberUsefulCloserPeers = n
		return nil
	}
}

func WithConcurrency[K kad.Key[K]](n int) Option[K] {
	return func(cfg *Config[K]) error {
		if n <= 0 {
			return fmt.Errorf("concurrency parameter must be positive")
		}
		cfg.Concurrency = n
		return nil
	}
}

func WithRequestTimeout[K kad.Key[K]](timeout time.Duration) Option[K] {
	return func(cfg *Config[K]) error {
		cfg.RequestTimeout = timeout
		return nil
	}
}

func WithPeerstoreTTL[K kad.Key[K]](ttl time.Duration) Option[K] {
	return func(cfg *Config[K]) error {
		cfg.PeerstoreTTL = ttl
		return nil
	}
}

func WithHandleResultsFunc[K kad.Key[K]](fn HandleResultFn[K]) Option[K] {
	return func(cfg *Config[K]) error {
		if fn == nil {
			return fmt.Errorf("HandleResultsFunc cannot be nil")
		}
		cfg.HandleResultsFunc = fn
		return nil
	}
}

func WithNotifyFailureFunc[K kad.Key[K]](fn NotifyFailureFn) Option[K] {
	return func(cfg *Config[K]) error {
		if fn == nil {
			return fmt.Errorf("NotifyFailureFunc cannot be nil")
		}
		cfg.NotifyFailureFunc = fn
		return nil
	}
}

func WithRoutingTable[K kad.Key[K]](rt routing.Table[K]) Option[K] {
	return func(cfg *Config[K]) error {
		if rt == nil {
			return fmt.Errorf("SimpleQuery option RoutingTable cannot be nil")
		}
		cfg.RoutingTable = rt
		return nil
	}
}

func WithEndpoint[K kad.Key[K]](ep endpoint.Endpoint[K]) Option[K] {
	return func(cfg *Config[K]) error {
		if ep == nil {
			return fmt.Errorf("SimpleQuery option Endpoint cannot be nil")
		}
		cfg.Endpoint = ep
		return nil
	}
}

func WithScheduler[K kad.Key[K]](sched scheduler.Scheduler) Option[K] {
	return func(cfg *Config[K]) error {
		if sched == nil {
			return fmt.Errorf("SimpleQuery option Scheduler cannot be nil")
		}
		cfg.Scheduler = sched
		return nil
	}
}
