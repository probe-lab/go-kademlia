package simplequery

import (
	"context"
	"fmt"
	"time"

	"github.com/probe-lab/go-kademlia/event"
	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/network/address"
	"github.com/probe-lab/go-kademlia/network/endpoint"
)

// Config is a structure containing all the options that can be used when
// constructing a SimpleQuery.
type Config[K kad.Key[K], A kad.Address[A]] struct {
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
	HandleResultsFunc HandleResultFn[K, A]
	// NotifyFailureFn is a function that is called when the query fails. It is
	// used to notify the user that the query failed.
	NotifyFailureFunc NotifyFailureFn

	// RoutingTable is the routing table used to find closer peers. It is
	// updated with newly discovered peers.
	RoutingTable kad.RoutingTable[K, kad.NodeID[K]]
	// Endpoint is the message endpoint used to send requests
	Endpoint endpoint.Endpoint[K, A]
	// Scheduler is the scheduler used to schedule events for the single worker
	Scheduler event.Scheduler
}

// Apply applies the SimpleQuery options to this Option
func (cfg *Config[K, A]) Apply(opts ...Option[K, A]) error {
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
type Option[K kad.Key[K], A kad.Address[A]] func(*Config[K, A]) error

// DefaultConfig is the default options for SimpleQuery. This option is always
// prepended to the list of options passed to the SimpleQuery constructor.
// Note that most of the fields are left empty, and must be filled by the user.
func DefaultConfig[K kad.Key[K], A kad.Address[A]](cfg *Config[K, A]) error {
	cfg.NumberUsefulCloserPeers = 20
	cfg.Concurrency = 10

	cfg.RequestTimeout = time.Second
	cfg.PeerstoreTTL = 30 * time.Minute

	cfg.HandleResultsFunc = func(ctx context.Context, id kad.NodeID[K],
		resp kad.Response[K, A],
	) (bool, []kad.NodeID[K]) {
		ids := make([]kad.NodeID[K], len(resp.CloserNodes()))
		for i, n := range resp.CloserNodes() {
			ids[i] = n.ID()
		}
		return false, ids
	}
	cfg.NotifyFailureFunc = func(context.Context) {}

	return nil
}

func WithProtocolID[K kad.Key[K], A kad.Address[A]](pid address.ProtocolID) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		cfg.ProtocolID = pid
		return nil
	}
}

func WithNumberUsefulCloserPeers[K kad.Key[K], A kad.Address[A]](n int) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		if n <= 0 {
			return fmt.Errorf("NumberUsefulCloserPeers must be positive")
		}
		cfg.NumberUsefulCloserPeers = n
		return nil
	}
}

func WithConcurrency[K kad.Key[K], A kad.Address[A]](n int) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		if n <= 0 {
			return fmt.Errorf("concurrency parameter must be positive")
		}
		cfg.Concurrency = n
		return nil
	}
}

func WithRequestTimeout[K kad.Key[K], A kad.Address[A]](timeout time.Duration) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		cfg.RequestTimeout = timeout
		return nil
	}
}

func WithPeerstoreTTL[K kad.Key[K], A kad.Address[A]](ttl time.Duration) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		cfg.PeerstoreTTL = ttl
		return nil
	}
}

func WithHandleResultsFunc[K kad.Key[K], A kad.Address[A]](fn HandleResultFn[K, A]) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		if fn == nil {
			return fmt.Errorf("HandleResultsFunc cannot be nil")
		}
		cfg.HandleResultsFunc = fn
		return nil
	}
}

func WithNotifyFailureFunc[K kad.Key[K], A kad.Address[A]](fn NotifyFailureFn) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		if fn == nil {
			return fmt.Errorf("NotifyFailureFunc cannot be nil")
		}
		cfg.NotifyFailureFunc = fn
		return nil
	}
}

func WithRoutingTable[K kad.Key[K], A kad.Address[A]](rt kad.RoutingTable[K, kad.NodeID[K]]) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		if rt == nil {
			return fmt.Errorf("SimpleQuery option RoutingTable cannot be nil")
		}
		cfg.RoutingTable = rt
		return nil
	}
}

func WithEndpoint[K kad.Key[K], A kad.Address[A]](ep endpoint.Endpoint[K, A]) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		if ep == nil {
			return fmt.Errorf("SimpleQuery option Endpoint cannot be nil")
		}
		cfg.Endpoint = ep
		return nil
	}
}

func WithScheduler[K kad.Key[K], A kad.Address[A]](sched event.Scheduler) Option[K, A] {
	return func(cfg *Config[K, A]) error {
		if sched == nil {
			return fmt.Errorf("SimpleQuery option Scheduler cannot be nil")
		}
		cfg.Scheduler = sched
		return nil
	}
}
