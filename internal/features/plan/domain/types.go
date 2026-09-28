// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:revive // The planner domain intentionally exposes its public data contracts.
package domain

import (
	"os"

	"github.com/task-otter/Taskotter/internal/features/state"
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/state/managed"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (

	// Snapshot provides module paths and store ref metadata needed for planning.
	Snapshot interface {
		ModuleDir(sourceModule string) string
		WorkspaceRoot() string
		SourceRef() string
		ResolvedCommit() string
		DefaultBranch() string
	}

	// TaskfileOps provides Taskfile operations needed to build a plan.
	TaskfileOps interface {
		NewRootTemplate() []byte
		RewriteIncludes(
			content []byte,
			destinations map[string]string,
			targetFolder string,
		) ([]byte, error)
		UpdateRootTaskfile(content []byte, input *RootUpdateInput) ([]byte, error)
	}

	// RootUpdateInput carries data for updating a root Taskfile.
	RootUpdateInput = struct {
		Tasks            []string
		TargetFolder     string
		RootTaskfileDir  string
		DestByTask       map[string]string
		ManagedTasks     []string
		ModuleTaskfiles  map[string][]byte
		GeneratedTasks   []GeneratedRootTask
		ManagedRootTasks []string
	}

	// GeneratedRootTask describes a TaskOtter-managed root task.
	GeneratedRootTask = struct {
		Name    string
		Modules []string
	}

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
		Metadata         state.Metadata
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
		Snapshot     Snapshot
		TaskfileOps  TaskfileOps
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
