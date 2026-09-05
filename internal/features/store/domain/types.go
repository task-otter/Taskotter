// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package domain holds store ref and snapshot types.
package domain

import (
	"fmt"
	"path/filepath"
)

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

const (
	defaultTaskfilesDir = "taskfiles"
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

// SourceRef returns the resolved source ref label from Ref.
func SourceRef(s *Snapshot) string {
	return s.Ref.SourceRef
}

// WorkspaceRoot returns the extracted store tree root in RootDir.
func WorkspaceRoot(s *Snapshot) string {
	return s.RootDir
}
