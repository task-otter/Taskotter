// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

// Close removes temporary snapshot files when present.
func Close(s *Snapshot) error {
	if s == nil || s.Cleanup == nil {
		return nil
	}

	err := s.Cleanup()
	if err != nil {
		return fmt.Errorf("clean up snapshot: %w", err)
	}

	return nil
}

// DefaultBranch returns the store repository default branch from Ref.
func DefaultBranch(s *Snapshot) string {
	return s.Ref.DefaultBranch
}

// ModuleDir returns the on-disk path for a source module directory under RootDir.
func ModuleDir(s *Snapshot, sourceModule string) string {
	return filepath.Join(s.RootDir, defaultTaskfilesDir, sourceModule)
}

// ResolvedCommit returns the resolved commit SHA from Ref.
func ResolvedCommit(s *Snapshot) string {
	return s.Ref.ResolvedCommit
}

// SemanticVersion normalizes a version tag for comparison, or returns empty for a non-version tag.
func SemanticVersion(tag string) string {
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	return semver.Canonical(tag)
}

// SourceRef returns the resolved source ref label from Ref.
func SourceRef(s *Snapshot) string {
	return s.Ref.SourceRef
}

// WorkspaceRoot returns the extracted store tree root in RootDir.
func WorkspaceRoot(s *Snapshot) string {
	return s.RootDir
}
