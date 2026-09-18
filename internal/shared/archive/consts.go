// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive

const (

	// MaxArchiveBytes is the maximum compressed archive size accepted for extraction.
	MaxArchiveBytes int64 = 100 * 1024 * 1024

	// MaxFileBytes is the maximum size of a single extracted file.
	MaxFileBytes int64 = 50 * 1024 * 1024

	// MaxTotalExtracted is the maximum total extracted payload size.
	MaxTotalExtracted int64 = 500 * 1024 * 1024

	dirPerm = 0o750

	maxTarFileMode int64 = 0o777

	errFmtWriteDirEntry = "write dir entry: %w"

	errFmtWriteEntry = "write entry: %w"

	errFmtEnsureValidTarPath = "ensure valid tar path: %w"
)
