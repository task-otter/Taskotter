// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	corruptLoadCase struct {
		load func(string, string) error
		rel  string
		want string
	}
)

const (
	testMetadataFileName       = "metadata.yml"
	testBadYAML                = ":"
	errExpectedCorruptMetadata = "expected corrupt metadata error"
	errExpectedCorruptLock     = "expected corrupt lock error"
	testTargetFolder           = "taskfiles"
	testMissingFile            = "missing.yml"
	testLockFile               = "lock.yml"
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

	if meta.TargetFolder != testTargetFolder || meta.LockFile != ".taskotter-lock.yml" {
		t.Fatalf("metadata = %#v", meta)
	}
}

// TestLoadMetadataCorruptFails verifies the behavior covered by this test.
func TestLoadMetadataCorruptFails(t *testing.T) {
	t.Parallel()

	assertCorruptLoadFails(t, &corruptLoadCase{
		rel:  testMetadataFileName,
		want: errExpectedCorruptMetadata,
		load: loadCorruptMetadata,
	})
}

// TestLoadMetadataMissingFileFails verifies the behavior covered by this test.
func TestLoadMetadataMissingFileFails(t *testing.T) {
	t.Parallel()

	meta, err := LoadMetadata(t.TempDir(), testMissingFile)
	iox.Discard(meta)

	if err == nil {
		t.Fatal("expected missing metadata error")
	}
}

// TestLoadLockReadsFile verifies the behavior covered by this test.
func TestLoadLockReadsFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := testLockFile
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
		Configuration: lockmodel.LockConfiguration{TargetFolder: testTargetFolder},
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

	differentSource := got.Source.Repository != want.Source.Repository
	differentTarget := got.Configuration.TargetFolder != want.Configuration.TargetFolder

	if differentSource || differentTarget {
		t.Fatalf("lock = %#v", got)
	}
}

// TestLoadLockCorruptFails verifies the behavior covered by this test.
func TestLoadLockCorruptFails(t *testing.T) {
	t.Parallel()

	assertCorruptLoadFails(t, &corruptLoadCase{
		rel:  testLockFile,
		want: errExpectedCorruptLock,
		load: loadCorruptLock,
	})
}

func loadCorruptMetadata(root, rel string) error {
	meta, err := LoadMetadata(root, rel)
	iox.Discard(meta)

	return fmt.Errorf(loadMetadataErrorFormat, err)
}

func loadCorruptLock(root, rel string) error {
	lock, err := LoadLock(root, rel)
	iox.Discard(lock)

	return fmt.Errorf("load lock: %w", err)
}

func assertCorruptLoadFails(t *testing.T, testCase *corruptLoadCase) {
	t.Helper()

	root := t.TempDir()

	err := os.WriteFile(filepath.Join(root, testCase.rel), []byte(testBadYAML), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}

	err = testCase.load(root, testCase.rel)
	if err == nil {
		t.Fatal(testCase.want)
	}
}

// TestLoadLockMissingFileFails verifies the behavior covered by this test.
func TestLoadLockMissingFileFails(t *testing.T) {
	t.Parallel()

	lock, err := LoadLock(t.TempDir(), testMissingFile)
	iox.Discard(lock)

	if err == nil {
		t.Fatal("expected missing lock error")
	}
}
