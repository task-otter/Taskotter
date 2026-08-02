// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"os"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/store"
)

// ModuleRecord maps a logical task to its source module and destination path.
type ModuleRecord struct {
	SourceModule      string
	DestinationModule string
	Path              string
}

// ManagedFile tracks a synced file in the lock file.
type ManagedFile struct {
	SourceModule      string
	DestinationModule string
	SourcePath        string
	Path              string
	SHA256            string
}

// LockFile is the on-disk sync state under the target folder.
type LockFile struct {
	Source struct {
		Repository       string
		RequestedVersion string
		SourceRef        string
		ResolvedCommit   string
		DefaultBranch    string
	}
	ResolvedModules struct {
		Requested    orderedRequested
		Dependencies []ModuleRecord
	}
	GeneratedRootTasks []string
	ManagedFiles       []ManagedFile
	Configuration      struct {
		TargetFolder       string
		NodePackageManager string
		NodeVersionManager string
		Tasks              []string
		IncludesDoc        bool
		SyncRoot           bool
	}
}

// Metadata points to the active lock file and configuration hash.
type Metadata struct {
	TargetFolder      string
	LockFile          string
	ConfigurationHash string
}

// FileEntry holds staged file bytes and permissions.
type FileEntry struct {
	Data []byte
	Mode os.FileMode
}

// Plan describes the sync diff and generated artifacts for one run.
type Plan struct {
	OldLock          *LockFile
	copyFileTo       func(string, FileEntry) error
	ModuleContents   map[string]map[string]FileEntry
	Requested        map[string]ModuleRecord
	Metadata         Metadata
	OldTargetFolder  string
	RootTaskfilePath string
	RootTaskfile     []byte
	Updated          []string
	Removed          []string
	Added            []string
	ManagedFiles     []ManagedFile
	StagePaths       []string
	Dependencies     []ModuleRecord
	Lock             LockFile
	Changed          bool
}

// SyncInput is the resolved store snapshot and module mapping for BuildPlan.
type SyncInput struct {
	Config       *config.Config
	Snapshot     *store.Snapshot
	Requested    map[string]ModuleRecord
	SourceToDest map[string]string
	DestByTask   map[string]string
	Dependencies []ModuleRecord
}

// SyncError reports user-facing sync planning failures.
type SyncError struct {
	Message string
}

func (e *SyncError) Error() string {
	return e.Message
}
