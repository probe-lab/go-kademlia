//go:build !go1.20

package testutil

import "testing"

func ReportTimePerKeyMetric(b *testing.B, n int) {
	// no-op
}
