// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

const (
	lockFileName = ".taskotter-lock.yml"

	legacyMetadataDirName = ".taskotter"

	legacyMetadataRelPath = ".taskotter/metadata.yml"

	errCreateStagingDir = "create staging directory: %w"

	errValidateRootTaskfile = "validate root Taskfile.yml: %w"

	errValidateLockFile = "validate lock file: %w"

	errValidateMetadata = "validate metadata: %w"

	errFmtCleanupStagingDir = "clean up staging directory %q: %w"

	errFmtRemoveObsoleteFile = "remove obsolete file %q: %w"

	errFmtRemoveStaleManaged = "remove stale managed file: %w"

	rootTaskfileName = "Taskfile.yml"
	fileModeRegular  = 0o644
	dirModePerm      = 0o755

	stagingTempPattern = ".taskotter-*"

	errFmtReadQuoted  = "read %q: %w"
	errFmtWriteQuoted = "write %q: %w"

	errDiscoverPreviousMetadata = "discover previous metadata: %w"

	errReadLockFile  = "read lock file %q: %w"
	errReadMetadata  = "read metadata %q: %w"
	errWalk          = "walk %q: %w"
	taskfilesDirName = "taskfiles"

	storeMetadataFileName = "metadata.yml"

	// storeMetadataSchema is the only metadata.yml schema this version understands.
	storeMetadataSchema = "taskotter.dev/taskfile-metadata/v1"

	fileUnchanged fileChangeKind = 0

	fileAdded = 1

	fileUpdated = 2

	docPolicySkip docPolicy = 0

	docPolicyInclude = 1

	// DocPolicySkip excludes README and docs/ paths from collected module files.
	DocPolicySkip DocPolicy = 0

	// DocPolicyInclude copies documentation paths alongside taskfiles.
	DocPolicyInclude = 1

	syncRootDisabled syncRootPolicy = 0

	syncRootEnabled = 1

	rootAbsent rootState = 0

	rootPresent = 1

	priorContentEmpty priorContent = 0

	priorContentExists = 1

	metadataNotCandidate metadataScanResult = 0

	metadataIsCandidate = 1

	yamlStagedSkip     yamlStagedKind = 0
	yamlStagedRoot     yamlStagedKind = 1
	yamlStagedLock     yamlStagedKind = 2
	yamlStagedMetadata yamlStagedKind = 3
)
