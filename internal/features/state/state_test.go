// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
)

const (
	testRequestedLint = "lint"
)

// TestStateCodecsRoundTrip verifies metadata and lock codecs preserve their values.
func TestStateCodecsRoundTrip(t *testing.T) {
	t.Parallel()
	t.Run(metadataLabel, testMetadataRoundTrip)
	t.Run("lock", testLockRoundTrip)
}

func testMetadataRoundTrip(t *testing.T) {
	t.Parallel()
	t.Helper()

	metadata := &Metadata{
		TargetFolder:      "taskfiles",
		LockFile:          "taskfiles/.taskotter-lock.yml",
		ConfigurationHash: "abc123",
	}

	metadataBytes := EncodeMetadata(metadata)

	decodedMetadata, err := DecodeMetadata(metadataBytes)
	if err != nil {
		t.Fatalf("DecodeMetadata() error = %v", err)
	}

	if *decodedMetadata != *metadata {
		t.Fatalf("metadata round trip = %#v, want %#v", decodedMetadata, metadata)
	}
}

func testLockRoundTrip(t *testing.T) {
	t.Parallel()
	t.Helper()

	lock := testLock()
	lockBytes := EncodeLock(lock)

	decodedLock, err := DecodeLock(lockBytes)
	if err != nil {
		t.Fatalf("DecodeLock() error = %v", err)
	}

	assertLockRoundTrip(t, decodedLock, lock)
}

func testLock() *lockmodel.LockFile {
	return &lockmodel.LockFile{
		Source: lockmodel.LockSource{Repository: "owner/store", ResolvedCommit: "deadbeef"},
		Requested: lockmodel.OrderedRequested{
			testRequestedLint: lockmodel.ModuleRecord{
				SourceModule: "eslint/node/pnpm",
				Path:         "taskfiles/eslint",
			},
		},
	}
}

func assertLockRoundTrip(t *testing.T, got, want *lockmodel.LockFile) {
	t.Helper()

	if got.Source.ResolvedCommit != want.Source.ResolvedCommit {
		t.Fatalf("lock commit = %q, want %q", got.Source.ResolvedCommit, want.Source.ResolvedCommit)
	}

	if got.Requested[testRequestedLint].Path != want.Requested[testRequestedLint].Path {
		t.Fatalf(
			"lock requested path = %q, want %q",
			got.Requested[testRequestedLint].Path,
			want.Requested[testRequestedLint].Path,
		)
	}
}
