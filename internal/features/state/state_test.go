// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import "testing"

func TestStateCodecsRoundTrip(t *testing.T) {
	t.Parallel()

	metadata := &Metadata{
		TargetFolder:      "taskfiles",
		LockFile:          "taskfiles/.taskotter-lock.yml",
		ConfigurationHash: "abc123",
	}

	metadataBytes, err := EncodeMetadata(metadata)
	if err != nil {
		t.Fatalf("EncodeMetadata() error = %v", err)
	}

	decodedMetadata, err := DecodeMetadata(metadataBytes)
	if err != nil {
		t.Fatalf("DecodeMetadata() error = %v", err)
	}

	if *decodedMetadata != *metadata {
		t.Fatalf("metadata round trip = %#v, want %#v", decodedMetadata, metadata)
	}

	lock := &LockFile{
		Source: LockSource{Repository: "owner/store", ResolvedCommit: "deadbeef"},
		Requested: OrderedRequested{
			"lint": ModuleRecord{
				SourceModule: "eslint/node/pnpm",
				Path:         "taskfiles/eslint",
			},
		},
	}

	lockBytes, err := EncodeLock(lock)
	if err != nil {
		t.Fatalf("EncodeLock() error = %v", err)
	}

	decodedLock, err := DecodeLock(lockBytes)
	if err != nil {
		t.Fatalf("DecodeLock() error = %v", err)
	}

	if decodedLock.Source.ResolvedCommit != lock.Source.ResolvedCommit {
		t.Fatalf(
			"lock commit = %q, want %q",
			decodedLock.Source.ResolvedCommit,
			lock.Source.ResolvedCommit,
		)
	}

	if decodedLock.Requested["lint"].Path != lock.Requested["lint"].Path {
		t.Fatalf(
			"lock requested path = %q, want %q",
			decodedLock.Requested["lint"].Path,
			lock.Requested["lint"].Path,
		)
	}
}
