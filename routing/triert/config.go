package triert

// Config holds configuration options for a TrieRT.
type Config struct {
	// KeyFilter defines the filter that is applied before a key is added to the table.
	// If nil, no filter is applied.
	KeyFilter KeyFilterFunc
}

// DefaultConfig returns a default configuration for a TrieRT.
func DefaultConfig() *Config {
	return &Config{
		KeyFilter: nil,
	}
}
