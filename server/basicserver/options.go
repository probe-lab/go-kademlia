package basicserver

import (
	"fmt"
	"time"
)

// Config is a structure containing all the options that can be used when
// constructing a SimpleRouting.
type Config struct {
	PeerstoreTTL              time.Duration
	NumberOfCloserPeersToSend int
}

// Apply applies the givkadsimservern options to this Option
func (cfg *Config) Apply(opts ...Option) error {
	for i, opt := range opts {
		if err := opt(cfg); err != nil {
			return fmt.Errorf("SimpleRouting option %d failed: %s", i, err)
		}
	}
	return nil
}

// Option type for SimpleRouting
type Option func(*Config) error

// DefaultConfig is the default options for SimpleRouting. This option is always
// prepended to the list of options passed to the SimpleRouting constructor.
var DefaultConfig = func(cfg *Config) error {
	cfg.PeerstoreTTL = 10 * time.Minute
	cfg.NumberOfCloserPeersToSend = 20

	return nil
}

func WithPeerstoreTTL(ttl time.Duration) Option {
	return func(cfg *Config) error {
		cfg.PeerstoreTTL = ttl
		return nil
	}
}

func WithNumberOfCloserPeersToSend(n int) Option {
	return func(cfg *Config) error {
		cfg.NumberOfCloserPeersToSend = n
		return nil
	}
}
