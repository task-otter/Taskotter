// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

type (
	// RefInfo describes a resolved store ref and commit.
	RefInfo = struct {
		Repository       string
		RequestedVersion string
		SourceRef        string
		ResolvedCommit   string
		DefaultBranch    string
	}

	// Snapshot holds an extracted store tree and module metadata.
	Snapshot = struct {
		Catalog map[string]struct{}
		Deps    map[string][]string
		Cleanup func() error
		Ref     RefInfo
		RootDir string
	}
)
