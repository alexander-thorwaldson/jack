//go:build testing

package testing

import (
	"errors"
	"testing"
)

func TestAssertNoError(t *testing.T) {
	// Should not fail with nil error.
	AssertNoError(t, nil)
}

func TestAssertError(t *testing.T) {
	// Should not fail with non-nil error.
	AssertError(t, errors.New("test error"))
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, 1, 1)
	AssertEqual(t, "foo", "foo")
	AssertEqual(t, true, true)
}
