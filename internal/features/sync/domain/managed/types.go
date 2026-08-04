// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package managed holds managed-file lock-domain types.
package managed

type (
	// File tracks a synced file in the lock file.
	File struct {
		SourceModule      string
		DestinationModule string
		SourcePath        string
		Path              string
		SHA256            string
	}
)
