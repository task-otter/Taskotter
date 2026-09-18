// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package testsupport provides shared testing helpers.
package testsupport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Lock serializes tests that replace process-wide seams or environment state.
func Lock() func() {
	lockPath := filepath.Join(os.TempDir(), "taskotter-test-lock")

	if !waitForLock(lockPath) {
		return func() {}
	}

	return func() {
		err := os.Remove(lockPath)

		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return
		}
	}
}

func waitForLock(path string) bool {
	for {
		acquired, err := acquireLock(path)

		if acquired {
			return true
		}

		if err != nil {
			return false
		}

		runtime.Gosched()
	}
}

func acquireLock(path string) (bool, error) {
	err := os.Mkdir(path, 0o700)

	if errors.Is(err, os.ErrExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("create test lock: %w", err)
	}

	return true, nil
}
