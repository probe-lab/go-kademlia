package query

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryPoolConfigValidate(t *testing.T) {
	t.Run("default is valid", func(t *testing.T) {
		cfg := DefaultQueryPoolConfig()
		require.NoError(t, cfg.Validate())
	})

	t.Run("clock is not nil", func(t *testing.T) {
		cfg := DefaultQueryPoolConfig()
		cfg.Clock = nil
		require.Error(t, cfg.Validate())
	})

	t.Run("concurrency positive", func(t *testing.T) {
		cfg := DefaultQueryPoolConfig()
		cfg.Concurrency = 0
		require.Error(t, cfg.Validate())
		cfg.Concurrency = -1
		require.Error(t, cfg.Validate())
	})

	t.Run("replication positive", func(t *testing.T) {
		cfg := DefaultQueryPoolConfig()
		cfg.Replication = 0
		require.Error(t, cfg.Validate())
		cfg.Replication = -1
		require.Error(t, cfg.Validate())
	})
}
