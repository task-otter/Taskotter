// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

const (
	rootTaskfileName = "Taskfile.yml"
	fileModeRegular  = 0o644
	dirModePerm      = 0o755

	errFmtReadQuoted  = "read %q: %w"
	errFmtWriteQuoted = "write %q: %w"

	errDiscoverPreviousMetadata = "discover previous metadata: %w"

	errMarshalLockFile = "marshal lock file: %w"

	errMarshalMetadata = "marshal metadata: %w"
)
