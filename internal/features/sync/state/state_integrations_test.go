// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/state"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	testMetadataFileName       = "metadata.yml"
	testBadYAML                = ":"
	errExpectedCorruptMetadata = "expected corrupt metadata error"
	errExpectedCorruptLock     = "expected corrupt lock error"
)

// TestLoadMetadataCorruptFails verifies malformed metadata YAML returns an error.
func TestLoadMetadataCorruptFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := testMetadataFileName

	err := os.WriteFile(filepath.Join(root, rel), []byte(testBadYAML), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := state.LoadMetadata(root, rel)
	iox.Discard(meta)

	if err == nil {
		t.Fatal(errExpectedCorruptMetadata)
	}
}

// TestLoadLockCorruptFails verifies malformed lock YAML returns an error.
func TestLoadLockCorruptFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := "lock.yml"

	err := os.WriteFile(filepath.Join(root, rel), []byte(testBadYAML), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	lock, err := state.LoadLock(root, rel)
	iox.Discard(lock)

	if err == nil {
		t.Fatal(errExpectedCorruptLock)
	}
}
