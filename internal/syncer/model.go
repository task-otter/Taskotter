// Package syncer plans and applies taskfile sync operations from the store into the workspace.
package syncer

import (
	"os"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/store"
)

// ModuleRecord maps a logical task to its source module and destination path.
type ModuleRecord struct {
	SourceModule      string `yaml:"source_module"`      //nolint:tagliatelle // lock file on-disk format
	DestinationModule string `yaml:"destination_module"` //nolint:tagliatelle // lock file on-disk format
	Path              string `yaml:"path,omitempty"`
}

// ManagedFile tracks a synced file in the lock file.
type ManagedFile struct {
	SourceModule      string `yaml:"source_module"`      //nolint:tagliatelle // lock file on-disk format
	DestinationModule string `yaml:"destination_module"` //nolint:tagliatelle // lock file on-disk format
	SourcePath        string `yaml:"source_path"`        //nolint:tagliatelle // lock file on-disk format
	Path              string `yaml:"path"`
	SHA256            string `yaml:"sha256"`
}

// LockFile is the on-disk sync state under the target folder.
type LockFile struct {
	Source struct {
		Repository       string `yaml:"repository"`
		RequestedVersion string `yaml:"requested_version"` //nolint:tagliatelle // lock file on-disk format
		SourceRef        string `yaml:"source_ref"`        //nolint:tagliatelle // lock file on-disk format
		ResolvedCommit   string `yaml:"resolved_commit"`   //nolint:tagliatelle // lock file on-disk format
		DefaultBranch    string `yaml:"default_branch"`    //nolint:tagliatelle // lock file on-disk format
	} `yaml:"source"`
	Configuration struct {
		TargetFolder       string   `yaml:"target_folder"` //nolint:tagliatelle // lock file on-disk format
		Tasks              []string `yaml:"tasks"`
		NodePackageManager string   `yaml:"node_package_manager"` //nolint:tagliatelle // lock file on-disk format
		NodeVersionManager string   `yaml:"node_version_manager"` //nolint:tagliatelle // lock file on-disk format
		IncludesDoc        bool     `yaml:"includes_doc"`         //nolint:tagliatelle // lock file on-disk format
		SyncRoot           bool     `yaml:"sync_root"`            //nolint:tagliatelle // lock file on-disk format
	} `yaml:"configuration"`
	ResolvedModules struct {
		Requested    orderedRequested `yaml:"requested"`
		Dependencies []ModuleRecord   `yaml:"dependencies"`
	} `yaml:"resolved_modules"` //nolint:tagliatelle // lock file on-disk format
	GeneratedRootTasks []string      `yaml:"generated_root_tasks,omitempty"` //nolint:tagliatelle // lock file on-disk format
	ManagedFiles       []ManagedFile `yaml:"managed_files"`                  //nolint:tagliatelle // lock file on-disk format
}

// Metadata points to the active lock file and configuration hash.
type Metadata struct {
	TargetFolder      string `yaml:"target_folder"`      //nolint:tagliatelle // metadata on-disk format
	LockFile          string `yaml:"lock_file"`          //nolint:tagliatelle // metadata on-disk format
	ConfigurationHash string `yaml:"configuration_hash"` //nolint:tagliatelle // metadata on-disk format
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
