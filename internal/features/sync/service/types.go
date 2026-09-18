// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"os"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	syncLock          = lockmodel.LockFile
	moduleRecord      = lockmodel.ModuleRecord
	managedFile       = managed.File
	generatedRootTask = ports.GeneratedRootTask
	rootUpdateInput   = ports.RootUpdateInput

	stagedFile = struct {
		finalRel string
		entry    domain.FileEntry
	}

	removeStaleFileArgs = struct {
		fsOps        *fileOps
		old          *managedFile
		current      map[string]struct{}
		workspace    string
		targetFolder string
	}

	// planArtifacts bundles the intermediate outputs produced while planning managed
	// files and the generated root Taskfile, before they're assembled into a plan.
	planArtifacts = struct {
		moduleContents     map[string]map[string]domain.FileEntry
		plannedFiles       []managedFile
		rootBytes          []byte
		newRoot            []byte
		generatedRootTasks []generatedRootTask
		rootState          rootState
	}

	appendManagedArgs = struct {
		mod        *moduleRecord
		contents   map[string]domain.FileEntry
		parentDocs map[string]struct{}
		destDirRel string
		planned    []managedFile
	}

	mergeParentDocsArgs = struct {
		fsOps      *fileOps
		collect    *collectModuleArgs
		contents   fMap
		parentDocs map[string]struct{}
		destRoot   string
	}

	fileEntryArgs = struct {
		ops          ports.TaskfileOps
		entry        os.DirEntry
		sourceToDest map[string]string
		sourceDir    string
		fromDest     string
		rel          string
		absPath      string
	}

	walkCollectArgs = struct {
		entry    os.DirEntry
		walkErr  error
		opts     *collectOptions
		contents map[string]domain.FileEntry
		absPath  string
	}

	// PrepareSyncInputArgs bundles inputs for PrepareSyncInput.
	PrepareSyncInputArgs = struct {
		Cfg         *config.Config
		Snapshot    ports.Snapshot
		TaskfileOps ports.TaskfileOps
		Resolutions []resolvesvc.Resolution
		DepSources  []string
	}

	buildReqArgs = struct {
		cfg *config.Config
		src map[string]string
		res []resolvesvc.Resolution
	}

	modRec = moduleRecord
	recMap = map[string]modRec

	// SnapshotAdapter adapts a store Snapshot to sync ports.Snapshot.
	SnapshotAdapter struct {
		snap *storedomain.Snapshot
	}

	storeTaskMetadata = struct {
		Schema        string   `yaml:"schema"`
		Module        string   `yaml:"module"`
		Taskfile      string   `yaml:"taskfile"`
		ExportedTasks []string `yaml:"exported_tasks"`
		Variants      []string `yaml:"variants"`
	}

	addCommonTasksInput struct {
		modulesByTask map[string][]string
		common        map[string]struct{}
		logicalTask   string
		exportedTasks []string
	}

	genRootTask = generatedRootTask

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

	buildRootPlanInput = struct {
		syncInput      *domain.SyncInput
		oldLock        *syncLock
		moduleContents mcMap
	}

	planManagedInput = struct {
		syncInput *domain.SyncInput
		oldLock   *syncLock
	}

	rootTaskfilesInput = struct {
		requested      map[string]moduleRecord
		moduleContents mcMap
	}

	diffInput = struct {
		fsOps        *fileOps
		plan         *domain.Plan
		workspace    string
		metadataPath string
		oldRoot      []byte
		plannedMeta  []byte
		syncRoot     syncRootPolicy
	}

	diffLists = struct {
		added   []string
		updated []string
		removed []string
	}

	previousState = struct {
		lock   *syncLock
		target string
	}

	rootPlanResult = struct {
		rootBytes          []byte
		newRoot            []byte
		generatedRootTasks []generatedRootTask
		rootState          rootState
	}

	stagingSession = struct {
		fsOps       *fileOps
		copyFile    func(string, *domain.FileEntry) error
		stagingRoot string
		staged      []stagedFile
	}

	collectOptions = struct {
		ops          ports.TaskfileOps
		fsOps        *fileOps
		sourceToDest map[string]string
		sourceDir    string
		fromDest     string
		docPolicy    docPolicy
	}

	// CollectOptions bundles inputs for CollectModuleFiles.
	CollectOptions = struct {
		fsOps        *fileOps
		TaskfileOps  ports.TaskfileOps
		SourceToDest map[string]string
		SourceDir    string
		FromDest     string
		DocPolicy    DocPolicy
	}

	moduleCollectArgs = struct {
		ops          ports.TaskfileOps
		fsOps        *fileOps
		entry        os.DirEntry
		sourceToDest map[string]string
		contents     map[string]domain.FileEntry
		sourceDir    string
		fromDest     string
		absPath      string
		docPolicy    docPolicy
	}

	lockPathResult = struct {
		lockPath string
		target   string
	}

	finalizePlanArgs = struct {
		fsOps        *fileOps
		plan         *domain.Plan
		meta         *domain.Metadata
		workspace    string
		metadataPath string
		rootBytes    []byte
		rootState    rootState
		syncRoot     syncRootPolicy
	}

	diffLockArgs = struct {
		fsOps     *fileOps
		plan      *domain.Plan
		workspace string
		lockPath  string
		lists     diffLists
	}

	diffMetadataArgs = struct {
		workspace    string
		metadataPath string
		plannedMeta  []byte
		lists        diffLists
	}

	stagePathsInput = struct {
		fsOps        *fileOps
		plan         *domain.Plan
		workspace    string
		metadataPath string
		syncRoot     syncRootPolicy
	}

	diffRootInput = struct {
		oldRoot  []byte
		newRoot  []byte
		rootPath string
		lists    diffLists
	}

	copyFileArgs = struct {
		fsOps *fileOps
		root  string
		rel   string
		dst   string
		mode  os.FileMode
	}

	finalizeTempArgs = struct {
		fsOps *fileOps
		tmp   *os.File
		path  string
		data  []byte
		mode  os.FileMode
	}

	modulePlanArgs = struct {
		fsOps          *fileOps
		syncInput      *domain.SyncInput
		mod            *moduleRecord
		oldLock        *syncLock
		moduleContents map[string]map[string]domain.FileEntry
		planned        []managedFile
	}

	collectModuleArgs = struct {
		fsOps      *fileOps
		syncInput  *domain.SyncInput
		mod        *moduleRecord
		sourceDir  string
		destDirRel string
	}

	validateStagedArgs = struct {
		rootPath string
		staged   []stagedFile
	}

	writeStagedArgs = struct {
		fsOps     *fileOps
		copyFile  func(string, *domain.FileEntry) error
		workspace string
		staged    []stagedFile
	}

	stagePlanArgs = struct {
		fsOps        *fileOps
		copyFile     func(string, *domain.FileEntry) error
		workspace    string
		targetFolder string
		staged       []stagedFile
	}

	buildRootArgs = struct {
		syncInput      *domain.SyncInput
		oldLock        *syncLock
		moduleContents map[string]map[string]domain.FileEntry
		rootBytes      []byte
	}

	resolveLockArgs = struct {
		cfg     *config.Config
		oldMeta *domain.Metadata
	}

	metadataWalkerArgs = struct {
		fsOps               *fileOps
		candidates          *[]string
		workspace           string
		currentMetadataPath string
	}

	metadataCandidateArgs = struct {
		fsOps               *fileOps
		entry               os.DirEntry
		workspace           string
		currentMetadataPath string
		abs                 string
	}

	applyStagedInput = struct {
		plan      *domain.Plan
		syncInput *domain.SyncInput
		workspace string
		session   stagingSession
	}

	prepareStagingInput = struct {
		fsOps     *fileOps
		plan      *domain.Plan
		syncInput *domain.SyncInput
		workspace string
	}

	stagePreparedInput = struct {
		fsOps     *fileOps
		plan      *domain.Plan
		syncInput *domain.SyncInput
		workspace string
		staged    []stagedFile
	}

	validateWriteStagedInput = struct {
		fsOps     *fileOps
		copyFile  func(string, *domain.FileEntry) error
		workspace string
		args      validateStagedArgs
	}

	assemblePlanInput = struct {
		syncInput *domain.SyncInput
		artifacts *planArtifacts
		meta      *domain.Metadata
		prev      previousState
		lock      syncLock
	}

	finalizeBuiltPlanInput = struct {
		syncInput *domain.SyncInput
		plan      *domain.Plan
		meta      *domain.Metadata
		artifacts *planArtifacts
	}

	modulePlanDirsInput = struct {
		fsOps     *fileOps
		syncInput *domain.SyncInput
		mod       *moduleRecord
		oldLock   *syncLock
		sourceDir string
	}

	rewriteModuleArgs = struct {
		ops          ports.TaskfileOps
		sourceToDest map[string]string
		sourceDir    string
		fromDest     string
		rel          string
		absPath      string
	}

	updateRootArgs = struct {
		moduleTaskfiles    map[string][]byte
		args               buildRootArgs
		generatedRootTasks []generatedRootTask
		managedTasks       []string
		managedRootTasks   []string
	}

	finishRootPlanInput = struct {
		syncInput      *domain.SyncInput
		oldLock        *syncLock
		moduleContents map[string]map[string]domain.FileEntry
		rootBytes      []byte
		rootStateVal   rootState
	}

	groupModulesInput = struct {
		requestedRecords map[string]moduleRecord
		metadata         map[string]storeTaskMetadata
		common           map[string]struct{}
		requested        []string
	}
)
