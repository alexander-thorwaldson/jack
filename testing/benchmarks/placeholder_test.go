//go:build testing

package benchmarks

import "testing"

// BenchmarkPlaceholder exists to ensure the benchmarks directory compiles.
// Replace with actual benchmarks as the package develops.
func BenchmarkPlaceholder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = i
	}
}
