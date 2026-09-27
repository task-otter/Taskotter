// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	testMetadataFileName       = "metadata.yml"
	testBadYAML                = ":"
	errExpectedCorruptMetadata = "expected corrupt metadata error"
	errExpectedCorruptLock     = "expected corrupt lock error"
)

// TestLoadMetadataReadsFile verifies the behavior covered by this test.
func TestLoadMetadataReadsFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := testMetadataFileName
	writeMetadataFixture(t, root, rel)

	meta, err := LoadMetadata(root, rel)
	if err != nil {
		t.Fatalf("LoadMetadata() error = %v", err)
	}

	assertMetadataFixture(t, meta)
}

func writeMetadataFixture(t *testing.T, root, rel string) {
	t.Helper()

	data := []byte(
		"target_folder: taskfiles\nlock_file: .taskotter-lock.yml\nconfiguration_hash: abc\n",
	)

	err := os.WriteFile(filepath.Join(root, rel), data, consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}

func assertMetadataFixture(t *testing.T, meta *domain.Metadata) {
	t.Helper()

	if meta.TargetFolder != "taskfiles" || meta.LockFile != ".taskotter-lock.yml" {
		t.Fatalf("metadata = %#v", meta)
	}
}

// TestLoadMetadataCorruptFails verifies the behavior covered by this test.
func TestLoadMetadataCorruptFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := testMetadataFileName

	err := os.WriteFile(filepath.Join(root, rel), []byte(testBadYAML), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := LoadMetadata(root, rel)
	iox.Discard(meta)

	if err == nil {
		t.Fatal(errExpectedCorruptMetadata)
	}
}

// TestLoadMetadataMissingFileFails verifies the behavior covered by this test.
func TestLoadMetadataMissingFileFails(t *testing.T) {
	t.Parallel()

	meta, err := LoadMetadata(t.TempDir(), "missing.yml")
	iox.Discard(meta)

	if err == nil {
		t.Fatal("expected missing metadata error")
	}
}

// TestLoadLockReadsFile verifies the behavior covered by this test.
func TestLoadLockReadsFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := "lock.yml"
	want := writeLockFixture(t, root, rel)

	lock, err := LoadLock(root, rel)
	if err != nil {
		t.Fatalf("LoadLock() error = %v", err)
	}

	assertLockFixture(t, lock, &want)
}

func writeLockFixture(t *testing.T, root, rel string) lockmodel.LockFile {
	t.Helper()

	want := lockmodel.LockFile{
		Source: lockmodel.LockSource{
			Repository: "task-otter/Taskotter-store",
			SourceRef:  "refs/heads/main",
		},
		Configuration: lockmodel.LockConfiguration{TargetFolder: "taskfiles"},
	}

	err := os.WriteFile(
		filepath.Join(root, rel),
		lockmodel.MarshalLock(&want),
		consts.FilePerm644,
	)
	if err != nil {
		t.Fatal(err)
	}

	return want
}

func assertLockFixture(t *testing.T, got, want *lockmodel.LockFile) {
	t.Helper()

	if got.Source.Repository != want.Source.Repository ||
		got.Configuration.TargetFolder != want.Configuration.TargetFolder {
		t.Fatalf("lock = %#v", got)
	}
}

// TestLoadLockCorruptFails verifies the behavior covered by this test.
func TestLoadLockCorruptFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := "lock.yml"

	err := os.WriteFile(filepath.Join(root, rel), []byte(testBadYAML), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	lock, err := LoadLock(root, rel)
	iox.Discard(lock)

	if err == nil {
		t.Fatal(errExpectedCorruptLock)
	}
}

// TestLoadLockMissingFileFails verifies the behavior covered by this test.
func TestLoadLockMissingFileFails(t *testing.T) {
	t.Parallel()

	lock, err := LoadLock(t.TempDir(), "missing.yml")
	iox.Discard(lock)

	if err == nil {
		t.Fatal("expected missing lock error")
	}
}
