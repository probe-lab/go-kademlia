package basicaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBasicAction(t *testing.T) {
	b := false
	ba := BasicAction(func(ctx context.Context) { b = true })
	ba.Run(context.Background())
	require.True(t, b)
}
