//go:build testing

package integration

import "testing"

// TestPlaceholder exists to ensure the integration directory compiles.
// Replace with actual integration tests as the package develops.
func TestPlaceholder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
}
