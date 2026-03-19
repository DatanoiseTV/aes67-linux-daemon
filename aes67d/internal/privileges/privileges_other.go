//go:build !linux

package privileges

import (
	"fmt"
	"runtime"
)

// Drop is not supported on non-Linux platforms.
func Drop(username string) error {
	return fmt.Errorf("privilege drop not supported on %s", runtime.GOOS)
}

// IsRoot always returns false on non-Linux platforms.
func IsRoot() bool {
	return false
}
