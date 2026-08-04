// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"os"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	fileChangeKind int

	docPolicy int

	// DocPolicy controls whether documentation files are included during module collection.
	DocPolicy int

	syncRootPolicy int

	rootState int

	priorContent int

	metadataScanResult int

	yamlStagedKind int

	fMap  map[string]domain.FileEntry
	mcMap map[string]map[string]domain.FileEntry

	storeTaskMetaMap map[string]storeTaskMetadata

	buildRootPlanInput struct {
		syncInput      *domain.SyncInput
		oldLock        *syncLock
		moduleContents mcMap
	}

	planManagedInput struct {
		syncInput *domain.SyncInput
		oldLock   *syncLock
	}

	rootTaskfilesInput struct {
		requested      map[string]moduleRecord
		moduleContents mcMap
	}

	diffInput struct {
		plan         *domain.Plan
		workspace    string
		metadataPath string
		oldRoot      []byte
		plannedMeta  []byte
		syncRoot     syncRootPolicy
	}

	diffLists struct {
		added   []string
		updated []string
		removed []string
	}

	previousState struct {
		lock   *syncLock
		target string
	}

	rootPlanResult struct {
		rootBytes          []byte
		newRoot            []byte
		generatedRootTasks []generatedRootTask
		rootState          rootState
	}

	stagingSession struct {
		copyFile    func(string, *domain.FileEntry) error
		stagingRoot string
		staged      []stagedFile
	}

	collectOptions struct {
		ops          ports.TaskfileOps
		sourceToDest map[string]string
		sourceDir    string
		docPolicy    docPolicy
	}

	// CollectOptions bundles inputs for CollectModuleFiles.
	CollectOptions struct {
		TaskfileOps  ports.TaskfileOps
		SourceToDest map[string]string
		SourceDir    string
		DocPolicy    DocPolicy
	}

	moduleCollectArgs struct {
		ops          ports.TaskfileOps
		entry        os.DirEntry
		sourceToDest map[string]string
		contents     map[string]domain.FileEntry
		sourceDir    string
		absPath      string
		docPolicy    docPolicy
	}

	lockPathResult struct {
		lockPath string
		target   string
	}

	finalizePlanArgs struct {
		plan         *domain.Plan
		meta         *domain.Metadata
		workspace    string
		metadataPath string
		rootBytes    []byte
		rootState    rootState
		syncRoot     syncRootPolicy
	}

	diffLockArgs struct {
		plan      *domain.Plan
		workspace string
		lockPath  string
		lists     diffLists
	}

	diffMetadataArgs struct {
		workspace    string
		metadataPath string
		plannedMeta  []byte
		lists        diffLists
	}

	stagePathsInput struct {
		plan         *domain.Plan
		workspace    string
		metadataPath string
		syncRoot     syncRootPolicy
	}

	diffRootInput struct {
		oldRoot  []byte
		newRoot  []byte
		rootPath string
		lists    diffLists
	}

	copyFileArgs struct {
		root string
		rel  string
		dst  string
		mode os.FileMode
	}

	finalizeTempArgs struct {
		tmp  *os.File
		path string
		data []byte
		mode os.FileMode
	}

	modulePlanArgs struct {
		syncInput      *domain.SyncInput
		mod            *moduleRecord
		oldLock        *syncLock
		moduleContents map[string]map[string]domain.FileEntry
		planned        []managedFile
	}

	collectModuleArgs struct {
		syncInput  *domain.SyncInput
		mod        *moduleRecord
		sourceDir  string
		destDirRel string
	}

	validateStagedArgs struct {
		rootPath string
		staged   []stagedFile
	}

	writeStagedArgs struct {
		copyFile  func(string, *domain.FileEntry) error
		workspace string
		staged    []stagedFile
	}

	stagePlanArgs struct {
		copyFile     func(string, *domain.FileEntry) error
		workspace    string
		targetFolder string
		staged       []stagedFile
	}

	buildRootArgs struct {
		syncInput      *domain.SyncInput
		oldLock        *syncLock
		moduleContents map[string]map[string]domain.FileEntry
		rootBytes      []byte
	}

	resolveLockArgs struct {
		cfg     *config.Config
		oldMeta *domain.Metadata
	}

	metadataWalkerArgs struct {
		candidates          *[]string
		workspace           string
		currentMetadataPath string
	}

	metadataCandidateArgs struct {
		entry               os.DirEntry
		workspace           string
		currentMetadataPath string
		abs                 string
	}

	applyStagedInput struct {
		plan      *domain.Plan
		syncInput *domain.SyncInput
		workspace string
		session   stagingSession
	}

	prepareStagingInput struct {
		plan      *domain.Plan
		syncInput *domain.SyncInput
		workspace string
	}

	stagePreparedInput struct {
		plan      *domain.Plan
		syncInput *domain.SyncInput
		workspace string
		staged    []stagedFile
	}

	validateWriteStagedInput struct {
		copyFile  func(string, *domain.FileEntry) error
		workspace string
		args      validateStagedArgs
	}

	assemblePlanInput struct {
		syncInput *domain.SyncInput
		artifacts *planArtifacts
		meta      *domain.Metadata
		prev      previousState
		lock      syncLock
	}

	finalizeBuiltPlanInput struct {
		syncInput *domain.SyncInput
		plan      *domain.Plan
		meta      *domain.Metadata
		artifacts *planArtifacts
	}

	modulePlanDirsInput struct {
		syncInput *domain.SyncInput
		mod       *moduleRecord
		oldLock   *syncLock
		sourceDir string
	}

	rewriteModuleArgs struct {
		ops          ports.TaskfileOps
		sourceToDest map[string]string
		sourceDir    string
		rel          string
		absPath      string
	}

	updateRootArgs struct {
		moduleTaskfiles    map[string][]byte
		args               buildRootArgs
		generatedRootTasks []generatedRootTask
		managedTasks       []string
		managedRootTasks   []string
	}

	finishRootPlanInput struct {
		syncInput      *domain.SyncInput
		oldLock        *syncLock
		moduleContents map[string]map[string]domain.FileEntry
		rootBytes      []byte
		rootStateVal   rootState
	}

	groupModulesInput struct {
		requestedRecords map[string]moduleRecord
		metadata         map[string]storeTaskMetadata
		common           map[string]struct{}
		requested        []string
	}
)

//nolint:goconst // distinct typed enums intentionally reuse 0/1/2
const (
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
)

//nolint:decorder,grouper // yamlStagedKind uses a dedicated iota block separate from typed enums above
const (
	yamlStagedSkip yamlStagedKind = iota

	yamlStagedRoot

	yamlStagedLock

	yamlStagedMetadata
)

func collectOptionsFrom(opts *CollectOptions) *collectOptions {
	return &collectOptions{
		ops:          opts.TaskfileOps,
		sourceDir:    opts.SourceDir,
		docPolicy:    docPolicyFromExported(opts.DocPolicy),
		sourceToDest: opts.SourceToDest,
	}
}

func docPolicyFromConfig(cfg *config.Config) docPolicy {
	if cfg.IncludesDoc {
		return docPolicyInclude
	}

	return docPolicySkip
}

func docPolicyFromExported(policy DocPolicy) docPolicy {
	if policy == DocPolicyInclude {
		return docPolicyInclude
	}

	return docPolicySkip
}

func syncRootFromConfig(cfg *config.Config) syncRootPolicy {
	if cfg.SyncRoot {
		return syncRootEnabled
	}

	return syncRootDisabled
}
