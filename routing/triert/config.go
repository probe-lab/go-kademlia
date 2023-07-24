package triert

import (
	"github.com/plprobelab/go-kademlia/kad"
)

// Config holds configuration options for a TrieRT.
type Config[T kad.Key[T]] struct {
	// KeyFilter defines the filter that is applied before a key is added to the table.
	// If nil, no filter is applied.
	KeyFilter KeyFilterFunc[T]
}

// DefaultConfig returns a default configuration for a TrieRT.
func DefaultConfig[T kad.Key[T]]() *Config[T] {
	return &Config[T]{
		KeyFilter: nil,
	}
}
