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
	storeMetadataSchema                = "taskotter.dev/taskfile-metadata/v1"
	fileUnchanged       fileChangeKind = iota
	fileAdded
	fileUpdated

	docPolicySkip docPolicy = iota
	docPolicyInclude

	// DocPolicySkip excludes README and docs/ paths from collected module files.
	DocPolicySkip DocPolicy = DocPolicy(docPolicySkip)

	// DocPolicyInclude copies documentation paths alongside taskfiles.
	DocPolicyInclude DocPolicy = DocPolicy(docPolicyInclude)

	syncRootDisabled syncRootPolicy = syncRootPolicy(docPolicySkip)
	syncRootEnabled  syncRootPolicy = syncRootPolicy(docPolicyInclude)

	rootAbsent  rootState = rootState(docPolicySkip)
	rootPresent rootState = rootState(docPolicyInclude)

	priorContentEmpty  priorContent = priorContent(docPolicySkip)
	priorContentExists priorContent = priorContent(docPolicyInclude)

	metadataNotCandidate metadataScanResult = metadataScanResult(docPolicySkip)
	metadataIsCandidate  metadataScanResult = metadataScanResult(docPolicyInclude)

	yamlStagedSkip yamlStagedKind = yamlStagedKind(docPolicySkip)
	yamlStagedRoot yamlStagedKind = iota + 1
	yamlStagedLock
	yamlStagedMetadata
)
