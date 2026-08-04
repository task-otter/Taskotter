// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package domain holds store ref and snapshot types.
package domain

import (
	"fmt"
	"path/filepath"

	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	// RefInfo describes a resolved store ref and commit.
	RefInfo struct {
		Repository       string
		RequestedVersion string
		SourceRef        string
		ResolvedCommit   string
		DefaultBranch    string
	}

	// Snapshot holds an extracted store tree and module metadata.
	Snapshot struct {
		Catalog map[string]struct{}
		Deps    map[string][]string
		Cleanup func() error
		Ref     RefInfo
		RootDir string
	}
)

// Close removes temporary snapshot files when present.
func (s *Snapshot) Close() error {
	if s.Cleanup == nil {
		return nil
	}

	err := s.Cleanup()
	if err != nil {
		return fmt.Errorf("clean up snapshot: %w", err)
	}

	return nil
}

// DefaultBranch returns the store repository default branch.
func (s *Snapshot) DefaultBranch() string {
	return s.Ref.DefaultBranch
}

// ModuleDir returns the on-disk path for a source module directory.
func (s *Snapshot) ModuleDir(sourceModule string) string {
	return filepath.Join(s.RootDir, config.DefaultTargetFolder, sourceModule)
}

// ResolvedCommit returns the resolved commit SHA.
func (s *Snapshot) ResolvedCommit() string {
	return s.Ref.ResolvedCommit
}

// SourceRef returns the resolved source ref label.
func (s *Snapshot) SourceRef() string {
	return s.Ref.SourceRef
}

// WorkspaceRoot returns the extracted store tree root.
func (s *Snapshot) WorkspaceRoot() string {
	return s.RootDir
}
