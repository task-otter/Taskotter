// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package testsupport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockAndAcquireLock(t *testing.T) {
	t.Parallel()

	unlock := Lock()
	unlock()

	path := filepath.Join(t.TempDir(), "lock")
	acquired, err := acquireLock(path)

	if err != nil || !acquired {
		t.Fatalf("acquireLock() = %v, %v", acquired, err)
	}

	acquired, err = acquireLock(path)

	if err != nil || acquired {
		t.Fatalf("existing acquireLock() = %v, %v", acquired, err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForLockReportsInvalidParent(t *testing.T) {
	t.Parallel()

	if waitForLock(filepath.Join(t.TempDir(), "missing", "lock")) {
		t.Fatal("invalid lock parent acquired")
	}
}
