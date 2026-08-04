// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package lockmodel holds lock-file domain types for TaskOtter sync state.
package lockmodel

import (
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
)

type (
	// LockSource records the store repository and resolved ref for a sync run.
	LockSource struct {
		Repository       string
		RequestedVersion string
		SourceRef        string
		ResolvedCommit   string
		DefaultBranch    string
	}

	// OrderedRequested preserves lock-file request order for YAML round-trips.
	OrderedRequested map[string]ModuleRecord

	// LockConfiguration captures consumer sync settings stored in the lock file.
	LockConfiguration struct {
		TargetFolder       string
		NodePackageManager string
		NodeVersionManager string
		Tasks              []string
		IncludesDoc        bool
		SyncRoot           bool
	}

	// LockFile is the on-disk sync state under the target folder.
	LockFile struct {
		Source             LockSource
		Requested          OrderedRequested
		Dependencies       []ModuleRecord
		GeneratedRootTasks []string
		ManagedFiles       []managed.File
		Configuration      LockConfiguration
	}

	// ModuleRecord maps a logical task to its source module and destination path.
	ModuleRecord struct {
		SourceModule      string
		DestinationModule string
		Path              string
	}
)
