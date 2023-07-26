package basicserver

import (
	"fmt"
	"time"
)

// Config is a structure containing all the options that can be used when
// constructing a BasicServer.
type Config struct {
	PeerstoreTTL            time.Duration
	NumberUsefulCloserPeers int
}

// Apply applies the BasicServer options to this Option
func (cfg *Config) Apply(opts ...Option) error {
	for i, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("BasicServer option %d failed: %s", i, err)
		}
	}
	return nil
}

// Option type for BasicServer
type Option func(*Config) error

// DefaultConfig is the default options for BasicServer. This option is always
// prepended to the list of options passed to the BasicServer constructor.
var DefaultConfig = func(cfg *Config) error {
	cfg.PeerstoreTTL = 30 * time.Minute
	cfg.NumberUsefulCloserPeers = 20

	return nil
}

func WithPeerstoreTTL(ttl time.Duration) Option {
	return func(cfg *Config) error {
		cfg.PeerstoreTTL = ttl
		return nil
	}
}

func WithNumberUsefulCloserPeers(n int) Option {
	return func(cfg *Config) error {
		cfg.NumberUsefulCloserPeers = n
		return nil
	}
}
