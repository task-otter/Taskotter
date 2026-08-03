// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/syncer"
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

	_, err = syncer.LoadMetadata(root, rel)
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

	_, err = syncer.LoadLock(root, rel)
	if err == nil {
		t.Fatal(errExpectedCorruptLock)
	}
}
