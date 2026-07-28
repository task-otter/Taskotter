// Package syncer plans and applies taskfile sync operations from the store into the workspace.
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
	Configuration struct {
		TargetFolder       string
		Tasks              []string
		NodePackageManager string
		NodeVersionManager string
		IncludesDoc        bool
		SyncRoot           bool
	}
	ResolvedModules struct {
		Requested    orderedRequested
		Dependencies []ModuleRecord
	}
	GeneratedRootTasks []string
	ManagedFiles       []ManagedFile
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
	Requested        map[string]ModuleRecord
	Dependencies     []ModuleRecord
	ManagedFiles     []ManagedFile
	ModuleContents   map[string]map[string]FileEntry
	RootTaskfile     []byte
	RootTaskfilePath string
	Lock             LockFile
	Metadata         Metadata
	Added            []string
	Updated          []string
	Removed          []string
	Changed          bool
	OldLock          *LockFile
	OldTargetFolder  string
	StagePaths       []string
	copyFileTo       func(string, FileEntry) error
}

// SyncInput is the resolved store snapshot and module mapping for BuildPlan.
type SyncInput struct {
	Config       *config.Config
	Snapshot     *store.Snapshot
	Requested    map[string]ModuleRecord
	Dependencies []ModuleRecord
	SourceToDest map[string]string
	DestByTask   map[string]string
}

// SyncError reports user-facing sync planning failures.
type SyncError struct {
	Message string
}

func (e *SyncError) Error() string {
	return e.Message
}
