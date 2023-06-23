package testaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntAction(t *testing.T) {
	a := IntAction(0)
	a.Run(context.Background())
	b := IntAction(1)

	require.NotEqual(t, a, b)
}

func TestFuncAction(t *testing.T) {
	a := NewFuncAction(0)
	b := NewFuncAction(1)
	require.NotEqual(t, a, b)

	a.Run(context.Background())
	require.True(t, a.Ran)
	require.False(t, b.Ran)
}
