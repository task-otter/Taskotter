// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"os"

	"github.com/task-otter/Taskotter/internal/features/root/ports"
	"github.com/task-otter/Taskotter/internal/features/state"
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/state/managed"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (

	// Metadata points to the active lock file and configuration hash.
	Metadata = state.Metadata

	// FileEntry holds staged file bytes and permissions.
	FileEntry = struct {
		Data []byte
		Mode os.FileMode
	}

	// Plan describes the sync diff and generated artifacts for one run.
	Plan = struct {
		Input            *SyncInput
		OldLock          *lockmodel.LockFile
		CopyFileTo       func(string, *FileEntry) error
		ModuleContents   map[string]map[string]FileEntry
		Requested        map[string]lockmodel.ModuleRecord
		Metadata         Metadata
		OldTargetFolder  string
		RootTaskfilePath string
		RootTaskfile     []byte
		Updated          []string
		Removed          []string
		Added            []string
		ManagedFiles     []managed.File
		StagePaths       []string
		Dependencies     []lockmodel.ModuleRecord
		Lock             lockmodel.LockFile
		Changed          bool
	}

	// SyncInput is the resolved store snapshot and module mapping for BuildPlan.
	SyncInput = struct {
		Config       *config.Config
		Snapshot     ports.Snapshot
		TaskfileOps  ports.TaskfileOps
		Requested    map[string]lockmodel.ModuleRecord
		SourceToDest map[string]string
		DestByTask   map[string]string
		Dependencies []lockmodel.ModuleRecord
	}

	// SyncError reports user-facing sync planning failures.
	SyncError string

	yamlDecodeTarget = struct {
		Out any
		Key string
	}
)
