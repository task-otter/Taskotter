// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

type (

	// planArtifacts bundles the intermediate outputs produced while planning managed
	// files and the generated root Taskfile, before they're assembled into a plan.
	planArtifacts struct {
		moduleContents     map[string]map[string]domain.FileEntry
		plannedFiles       []managedFile
		rootBytes          []byte
		newRoot            []byte
		generatedRootTasks []generatedRootTask
		rootState          rootState
	}

	appendManagedArgs struct {
		mod        *moduleRecord
		contents   map[string]domain.FileEntry
		parentDocs map[string]struct{}
		destDirRel string
		planned    []managedFile
	}

	fileEntryArgs struct {
		ops          ports.TaskfileOps
		entry        os.DirEntry
		sourceToDest map[string]string
		sourceDir    string
		fromDest     string
		rel          string
		absPath      string
	}

	walkCollectArgs struct {
		entry    os.DirEntry
		walkErr  error
		opts     *collectOptions
		contents map[string]domain.FileEntry
		absPath  string
	}
)

var errTaskfileOpsNotConfigured = errors.New("taskfile ops not configured")

// BuildPlan computes the sync diff and generated artifacts for syncInput.
func BuildPlan(syncInput *domain.SyncInput) (*domain.Plan, error) {
	prev, err := loadPreviousState(syncInput.Config.Workspace, syncInput.Config)
	if err != nil {
		return nil, fmt.Errorf("load previous state: %w", err)
	}

	plan, err := buildPlanFromState(syncInput, &prev)
	if err != nil {
		return nil, fmt.Errorf("build plan from state: %w", err)
	}

	return plan, nil
}

// CollectModuleFiles scans a module source directory and returns syncable file entries.
func CollectModuleFiles(opts *CollectOptions) (map[string]domain.FileEntry, error) {
	contents, err := scanModuleFiles(collectOptionsFrom(opts))
	if err != nil {
		return nil, fmt.Errorf("scan module files: %w", err)
	}

	return contents, nil
}

func accumulateModulePlan(args *modulePlanArgs) ([]managedFile, error) {
	contents, managed, err := planModuleFiles(args)
	if err != nil {
		return nil, fmt.Errorf("plan module files: %w", err)
	}

	args.moduleContents[args.mod.SourceModule] = contents

	return append(args.planned, managed...), nil
}

func anyChanges(lists *diffLists) bool {
	return len(lists.added) > consts.IndexZero || len(lists.updated) > consts.IndexZero ||
		len(lists.removed) > consts.IndexZero
}

func emptyRootPlanResult() rootPlanResult {
	return rootPlanResult{
		rootBytes:          nil,
		newRoot:            nil,
		generatedRootTasks: nil,
		rootState:          rootAbsent,
	}
}

func appendManagedFiles(args *appendManagedArgs) []managedFile {
	for rel := range args.contents {
		entry := args.contents[rel]
		sum := sha256.Sum256(entry.Data)

		args.planned = append(args.planned, managedFile{
			SourceModule:      args.mod.SourceModule,
			DestinationModule: args.mod.DestinationModule,
			SourcePath:        managedSourcePath(args, rel),
			Path:              pathutil.JoinRelative(args.destDirRel, rel),
			SHA256:            hex.EncodeToString(sum[:]),
		})
	}

	return args.planned
}

func managedSourcePath(args *appendManagedArgs, rel string) string {
	sourceModule := args.mod.SourceModule

	if _, fromParent := args.parentDocs[rel]; fromParent {
		sourceModule = args.mod.DestinationModule
	}

	return pathutil.JoinRelative(taskfilesDirName, sourceModule, rel)
}

func applyDiffResults(plan *domain.Plan, lists *diffLists, input *stagePathsInput) {
	plan.Added = lists.added
	plan.Updated = lists.updated
	plan.Removed = lists.removed
	plan.Changed = anyChanges(lists)
	plan.StagePaths = buildStagePaths(input)
}

func assemblePlan(input *assemblePlanInput) *domain.Plan {
	return &domain.Plan{
		Requested:        input.syncInput.Requested,
		Dependencies:     input.syncInput.Dependencies,
		ManagedFiles:     input.artifacts.plannedFiles,
		ModuleContents:   input.artifacts.moduleContents,
		RootTaskfile:     input.artifacts.newRoot,
		RootTaskfilePath: input.syncInput.Config.RootTaskfile,
		Lock:             input.lock,
		Metadata:         *input.meta,
		OldLock:          input.prev.lock,
		OldTargetFolder:  input.prev.target,
		Updated:          nil,
		Removed:          nil,
		Added:            nil,
		StagePaths:       nil,
		Changed:          false,
		CopyFileTo:       nil,
	}
}

func buildFileEntry(args *fileEntryArgs) (domain.FileEntry, error) {
	info, err := args.entry.Info()
	if err != nil {
		return domain.FileEntry{}, fmt.Errorf("file info for %q: %w", args.absPath, err)
	}

	data, err := readAndMaybeRewriteModuleFile(&rewriteModuleArgs{
		ops:          args.ops,
		sourceDir:    args.sourceDir,
		fromDest:     args.fromDest,
		rel:          args.rel,
		absPath:      args.absPath,
		sourceToDest: args.sourceToDest,
	})
	if err != nil {
		return domain.FileEntry{}, fmt.Errorf("read/rewrite module file: %w", err)
	}

	return domain.FileEntry{Data: data, Mode: preserveMode(info.Mode())}, nil
}

func buildLock(in *domain.SyncInput, files []managedFile, gen []generatedRootTask) syncLock {
	var lock syncLock

	setLockSource(&lock, in)
	setLockConfiguration(&lock, in.Config)
	setLockResolvedModules(&lock, in)

	lock.GeneratedRootTasks = generatedRootTaskNames(gen)
	lock.ManagedFiles = files

	return lock
}

func buildPlanFromState(syncInput *domain.SyncInput, prev *previousState) (*domain.Plan, error) {
	artifacts, err := planAllFiles(syncInput, prev.lock)
	if err != nil {
		return nil, fmt.Errorf("plan all files: %w", err)
	}

	plan, meta := newPlanFromInputs(syncInput, &artifacts, prev)

	planResult, err := finalizeBuiltPlan(&finalizeBuiltPlanInput{
		syncInput: syncInput,
		plan:      plan,
		meta:      meta,
		artifacts: &artifacts,
	})
	if err != nil {
		return nil, fmt.Errorf("finalize plan plan: %w", err)
	}

	return planResult, nil
}

func finalizeBuiltPlan(input *finalizeBuiltPlanInput) (*domain.Plan, error) {
	planResult, err := finalizePlanDiff(&finalizePlanArgs{
		plan:         input.plan,
		workspace:    input.syncInput.Config.Workspace,
		rootBytes:    input.artifacts.rootBytes,
		rootState:    input.artifacts.rootState,
		syncRoot:     syncRootFromConfig(input.syncInput.Config),
		meta:         input.meta,
		metadataPath: input.syncInput.Config.MetadataPath(),
	})
	if err != nil {
		return nil, fmt.Errorf("finalize plan diff: %w", err)
	}

	return planResult, nil
}

func buildRootPlanResult(input *buildRootPlanInput) (rootPlanResult, error) {
	finishInput, err := readRootPlanFinishInput(input)
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("read root plan finish input: %w", err)
	}

	rootResult, err := finishRootPlanResult(finishInput)
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("finish root plan result: %w", err)
	}

	return rootResult, nil
}

func readRootPlanFinishInput(input *buildRootPlanInput) (*finishRootPlanInput, error) {
	rootBytes, rootStateVal, err := readRootTaskfile(
		input.syncInput.TaskfileOps,
		input.syncInput.Config.Workspace,
		input.syncInput.Config.RootTaskfile,
	)
	if err != nil {
		return nil, fmt.Errorf("read root taskfile: %w", err)
	}

	return &finishRootPlanInput{
		syncInput:      input.syncInput,
		oldLock:        input.oldLock,
		moduleContents: input.moduleContents,
		rootBytes:      rootBytes,
		rootStateVal:   rootStateVal,
	}, nil
}

func buildRootTaskfile(args *buildRootArgs) (root []byte, tasks []generatedRootTask, err error) {
	inputs, err := prepareRootTaskfileInputs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare root taskfile inputs: %w", err)
	}

	rootBytes, generatedRootTasks, err := updateRootTaskfile(inputs)
	if err != nil {
		return nil, nil, fmt.Errorf("update root taskfile: %w", err)
	}

	return rootBytes, generatedRootTasks, nil
}

func rootTaskfileUpdateArgs(args *buildRootArgs, storeMetadata storeTaskMetaMap) *updateRootArgs {
	managedTasks, managedRootTasks := resolveManagedTasks(&planManagedInput{
		syncInput: args.syncInput, oldLock: args.oldLock,
	})
	moduleTaskfiles := collectModuleRootTaskfiles(&rootTaskfilesInput{
		requested: args.syncInput.Requested, moduleContents: args.moduleContents,
	})
	generatedRootTasks := buildGeneratedRootTasks(&groupModulesInput{
		requested: args.syncInput.Config.Tasks, requestedRecords: args.syncInput.Requested,
		metadata: storeMetadata, common: nil,
	})

	return &updateRootArgs{
		args:               *args,
		generatedRootTasks: generatedRootTasks,
		managedTasks:       managedTasks,
		managedRootTasks:   managedRootTasks,
		moduleTaskfiles:    moduleTaskfiles,
	}
}

func prepareRootTaskfileInputs(args *buildRootArgs) (*updateRootArgs, error) {
	storeMetadata, err := loadStoreTaskMetadata(args.syncInput.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("load store task metadata: %w", err)
	}

	return rootTaskfileUpdateArgs(args, storeMetadata), nil
}

func collectAndTrackModuleFiles(args *collectModuleArgs) (fMap, []managedFile, error) {
	policy := docPolicyFromConfig(args.syncInput.Config)

	contents, err := scanModuleFiles(&collectOptions{
		ops:          args.syncInput.TaskfileOps,
		sourceDir:    args.sourceDir,
		fromDest:     args.mod.DestinationModule,
		docPolicy:    policy,
		sourceToDest: args.syncInput.SourceToDest,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("collect module files: %w", err)
	}

	parentDocs, err := mergeLogicalRootDocs(args, contents, policy)
	if err != nil {
		return nil, nil, fmt.Errorf("merge logical root docs: %w", err)
	}

	managed := appendManagedFiles(&appendManagedArgs{
		planned:    nil,
		mod:        args.mod,
		destDirRel: args.destDirRel,
		contents:   contents,
		parentDocs: parentDocs,
	})

	return contents, managed, nil
}

func mergeLogicalRootDocs(
	args *collectModuleArgs,
	contents fMap,
	policy docPolicy,
) (map[string]struct{}, error) {
	parentDocs := make(map[string]struct{})

	if policy != docPolicyInclude {
		return parentDocs, nil
	}

	destRoot := args.syncInput.Snapshot.ModuleDir(args.mod.DestinationModule)
	if filepath.Clean(destRoot) == filepath.Clean(args.sourceDir) {
		return parentDocs, nil
	}

	err := mergeParentDocFiles(args, destRoot, contents, parentDocs)
	if err != nil {
		return nil, fmt.Errorf("merge parent doc files: %w", err)
	}

	return parentDocs, nil
}

func mergeParentDocFiles(
	args *collectModuleArgs,
	destRoot string,
	contents fMap,
	parentDocs map[string]struct{},
) error {
	info, err := os.Stat(destRoot)
	iox.Discard(info)

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("stat logical root %q: %w", destRoot, err)
	}

	rootContents, err := scanModuleFiles(&collectOptions{
		ops:          args.syncInput.TaskfileOps,
		sourceDir:    destRoot,
		fromDest:     args.mod.DestinationModule,
		docPolicy:    docPolicyInclude,
		sourceToDest: args.syncInput.SourceToDest,
	})
	if err != nil {
		return fmt.Errorf("scan logical root %q: %w", destRoot, err)
	}

	for rel, entry := range rootContents {
		if !pathutil.IsDocPath(rel) {
			continue
		}

		contents[rel] = entry
		parentDocs[rel] = struct{}{}
	}

	return nil
}

func collectModuleFile(args *moduleCollectArgs) error {
	rel, err := relSlashPath(args.sourceDir, args.absPath)
	if err != nil {
		return fmt.Errorf("rel slash path: %w", err)
	}

	if shouldSkipModuleFile(rel, args.docPolicy) {
		return nil
	}

	err = storeCollectedModuleFile(args, rel)
	if err != nil {
		return fmt.Errorf("store collected module file: %w", err)
	}

	return nil
}

func collectModuleRootTaskfiles(in *rootTaskfilesInput) map[string][]byte {
	moduleTaskfiles := make(map[string][]byte, len(in.requested))

	for task := range in.requested {
		rec := in.requested[task]
		files, ok := in.moduleContents[rec.SourceModule]

		if !ok {
			continue
		}

		if entry, hasRoot := files[rootTaskfileName]; hasRoot {
			moduleTaskfiles[task] = entry.Data
		}
	}

	return moduleTaskfiles
}

func collectModuleWalkFunc(
	opts *collectOptions,
	contents map[string]domain.FileEntry,
) fs.WalkDirFunc {
	return func(absPath string, entry os.DirEntry, walkErr error) error {
		return walkCollectModuleFile(&walkCollectArgs{
			opts:     opts,
			contents: contents,
			absPath:  absPath,
			entry:    entry,
			walkErr:  walkErr,
		})
	}
}

func walkCollectModuleFile(args *walkCollectArgs) error {
	if args.walkErr != nil {
		return fmt.Errorf(errWalk, args.absPath, args.walkErr)
	}

	if args.entry.IsDir() {
		return nil
	}

	err := collectWalkedModuleFile(args)
	if err != nil {
		return fmt.Errorf("walk collect module file: %w", err)
	}

	return nil
}

func collectWalkedModuleFile(args *walkCollectArgs) error {
	err := collectModuleFile(&moduleCollectArgs{
		ops:          args.opts.ops,
		sourceDir:    args.opts.sourceDir,
		fromDest:     args.opts.fromDest,
		absPath:      args.absPath,
		entry:        args.entry,
		docPolicy:    args.opts.docPolicy,
		sourceToDest: args.opts.sourceToDest,
		contents:     args.contents,
	})
	if err != nil {
		return fmt.Errorf("collect module file %q: %w", args.absPath, err)
	}

	return nil
}

func ensureSourceDirExists(sourceDir string, mod *moduleRecord) error {
	info, err := os.Stat(sourceDir)
	iox.Discard(info)

	if err != nil {
		return domain.SyncError(
			fmt.Sprintf("source module directory %q does not exist", mod.SourceModule),
		)
	}

	return nil
}

func finalizePlanDiff(args *finalizePlanArgs) (*domain.Plan, error) {
	lists, err := diffFiles(&diffInput{
		plan:         args.plan,
		workspace:    args.workspace,
		oldRoot:      oldRootForDiffing(args.rootBytes, args.rootState),
		syncRoot:     args.syncRoot,
		metadataPath: args.metadataPath,
		plannedMeta:  mustMarshalMetadata(args.meta),
	})
	if err != nil {
		return nil, fmt.Errorf("diff files: %w", err)
	}

	applyDiffResults(args.plan, &lists, &stagePathsInput{
		plan:         args.plan,
		workspace:    args.workspace,
		metadataPath: args.metadataPath,
		syncRoot:     args.syncRoot,
	})

	return args.plan, nil
}

func finishRootPlanResult(input *finishRootPlanInput) (rootPlanResult, error) {
	newRoot, generatedRootTasks, err := buildRootTaskfile(&buildRootArgs{
		syncInput:      input.syncInput,
		oldLock:        input.oldLock,
		moduleContents: input.moduleContents,
		rootBytes:      input.rootBytes,
	})
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("build root taskfile: %w", err)
	}

	return rootPlanResult{
		rootBytes:          input.rootBytes,
		rootState:          input.rootStateVal,
		newRoot:            newRoot,
		generatedRootTasks: generatedRootTasks,
	}, nil
}

func generatedRootTaskNames(generated []generatedRootTask) []string {
	names := make([]string, consts.IndexZero, len(generated))

	for i := range generated {
		names = append(names, generated[i].Name)
	}

	slices.Sort(names)

	return names
}

func isDestinationManaged(oldLock *syncLock, mod *moduleRecord) bool {
	if oldLock == nil {
		return false
	}

	for i := range oldLock.ManagedFiles {
		if oldLock.ManagedFiles[i].DestinationModule == mod.DestinationModule {
			return true
		}
	}

	return false
}

func mustMarshalMetadata(meta *domain.Metadata) []byte {
	data, err := MarshalMetadata(meta)
	if err != nil {
		return nil
	}

	return data
}

func newPlanFromInputs(
	syncIn *domain.SyncInput,
	art *planArtifacts,
	prev *previousState,
) (*domain.Plan, *domain.Metadata) {
	lock := buildLock(syncIn, art.plannedFiles, art.generatedRootTasks)
	meta := newPlanMetadata(syncIn)

	return assemblePlan(&assemblePlanInput{
		syncInput: syncIn,
		artifacts: art,
		prev:      *prev,
		lock:      lock,
		meta:      meta,
	}), meta
}

func newPlanMetadata(syncInput *domain.SyncInput) *domain.Metadata {
	return &domain.Metadata{
		TargetFolder:      syncInput.Config.TargetFolder,
		LockFile:          syncInput.Config.LockFilePath(),
		ConfigurationHash: syncInput.Config.ConfigurationHash,
	}
}

func oldRootForDiffing(rootBytes []byte, state rootState) []byte {
	if state == rootAbsent {
		return nil
	}

	return rootBytes
}

func artifactsFromRootPlan(files []managedFile, mc mcMap, root *rootPlanResult) planArtifacts {
	return planArtifacts{
		plannedFiles:       files,
		moduleContents:     mc,
		rootBytes:          root.rootBytes,
		rootState:          root.rootState,
		newRoot:            root.newRoot,
		generatedRootTasks: root.generatedRootTasks,
	}
}

func planAllFiles(syncInput *domain.SyncInput, oldLock *syncLock) (planArtifacts, error) {
	plannedFiles, moduleContents, err := planManagedFiles(&planManagedInput{
		syncInput: syncInput, oldLock: oldLock,
	})
	if err != nil {
		return planArtifacts{}, fmt.Errorf("plan managed files: %w", err)
	}

	rootResult, err := planRootTaskfile(&buildRootPlanInput{
		syncInput: syncInput, oldLock: oldLock, moduleContents: moduleContents,
	})
	if err != nil {
		return planArtifacts{}, fmt.Errorf("plan root taskfile: %w", err)
	}

	return artifactsFromRootPlan(plannedFiles, moduleContents, &rootResult), nil
}

func planAllModules(allModules []moduleRecord, args *modulePlanArgs) ([]managedFile, error) {
	var planned []managedFile

	for i := range allModules {
		mod := &allModules[i]

		var err error

		planned, err = accumulateModulePlan(&modulePlanArgs{
			syncInput:      args.syncInput,
			mod:            mod,
			oldLock:        args.oldLock,
			moduleContents: args.moduleContents,
			planned:        planned,
		})
		if err != nil {
			return nil, fmt.Errorf("accumulate module plan for %q: %w", mod.SourceModule, err)
		}
	}

	return planned, nil
}

func planManagedFiles(args *planManagedInput) ([]managedFile, mcMap, error) {
	allModules := sortedModules(args.syncInput)
	moduleContents := make(mcMap)

	planned, err := planAllModules(allModules, &modulePlanArgs{
		syncInput:      args.syncInput,
		mod:            nil,
		oldLock:        args.oldLock,
		moduleContents: moduleContents,
		planned:        nil,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("plan all modules: %w", err)
	}

	sortManagedFiles(planned)

	return planned, moduleContents, nil
}

func planModuleFiles(args *modulePlanArgs) (fMap, []managedFile, error) {
	collectArgs, err := moduleCollectArgsForPlan(args)
	if err != nil {
		return nil, nil, fmt.Errorf("module collect args for plan: %w", err)
	}

	contents, managed, err := collectAndTrackModuleFiles(collectArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("collect and track module files: %w", err)
	}

	return contents, managed, nil
}

func moduleCollectArgsForPlan(args *modulePlanArgs) (*collectModuleArgs, error) {
	sourceDir := args.syncInput.Snapshot.ModuleDir(args.mod.SourceModule)

	destDirRel, err := prepareModulePlanDirs(&modulePlanDirsInput{
		syncInput: args.syncInput,
		mod:       args.mod,
		oldLock:   args.oldLock,
		sourceDir: sourceDir,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare module plan dirs: %w", err)
	}

	return &collectModuleArgs{
		syncInput: args.syncInput, mod: args.mod, sourceDir: sourceDir, destDirRel: destDirRel,
	}, nil
}

func prepareModulePlanDirs(input *modulePlanDirsInput) (string, error) {
	err := ensureSourceDirExists(input.sourceDir, input.mod)
	if err != nil {
		return consts.Empty, fmt.Errorf("ensure source dir exists: %w", err)
	}

	destDirRel, err := validateModuleDestination(&modulePlanArgs{
		syncInput: input.syncInput, mod: input.mod, oldLock: input.oldLock,
		moduleContents: nil, planned: nil,
	})
	if err != nil {
		return consts.Empty, fmt.Errorf("validate module destination: %w", err)
	}

	return destDirRel, nil
}

func planRootTaskfile(input *buildRootPlanInput) (rootPlanResult, error) {
	if !input.syncInput.Config.SyncRoot {
		return emptyRootPlanResult(), nil
	}

	rootResult, err := buildRootPlanResult(input)
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("build root plan result: %w", err)
	}

	return rootResult, nil
}

func readAndMaybeRewriteModuleFile(args *rewriteModuleArgs) ([]byte, error) {
	data, err := pathutil.ReadRelativeFile(args.sourceDir, args.rel)
	if err != nil {
		return nil, fmt.Errorf("read module file %q: %w", args.absPath, err)
	}

	rewritten, err := maybeRewriteRootTaskfile(args, data)
	if err != nil {
		return nil, fmt.Errorf("rewrite module file %q: %w", args.absPath, err)
	}

	return rewritten, nil
}

func maybeRewriteRootTaskfile(args *rewriteModuleArgs, data []byte) ([]byte, error) {
	if args.rel != rootTaskfileName || args.ops == nil {
		return data, nil
	}

	rewritten, err := args.ops.RewriteIncludes(data, args.sourceToDest, args.fromDest)
	if err != nil {
		return nil, fmt.Errorf("rewrite includes in %q: %w", args.absPath, err)
	}

	return rewritten, nil
}

func readRootTaskfile(
	ops ports.TaskfileOps,
	workspace, rootPath string,
) ([]byte, rootState, error) {
	rootBytes, err := pathutil.ReadRelativeFile(workspace, rootPath)
	if err == nil {
		return rootBytes, rootPresent, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, rootAbsent, fmt.Errorf("read root Taskfile.yml: %w", err)
	}

	template, templateErr := rootTemplateOrError(ops)
	if templateErr != nil {
		return nil, rootAbsent, fmt.Errorf("root taskfile template: %w", templateErr)
	}

	return template, rootAbsent, nil
}

func rootTemplateOrError(ops ports.TaskfileOps) ([]byte, error) {
	if ops == nil {
		return nil, errTaskfileOpsNotConfigured
	}

	return ops.NewRootTemplate(), nil
}

func relSlashPath(sourceDir, absPath string) (string, error) {
	rel, err := filepath.Rel(sourceDir, absPath)
	if err != nil {
		return consts.Empty, fmt.Errorf("rel path for %q: %w", absPath, err)
	}

	return filepath.ToSlash(rel), nil
}

func resolveManagedTasks(args *planManagedInput) (managedTasks, managedRootTasks []string) {
	if args.oldLock != nil {
		return args.oldLock.Configuration.Tasks, args.oldLock.GeneratedRootTasks
	}

	return args.syncInput.Config.Tasks, nil
}

func scanModuleFiles(opts *collectOptions) (map[string]domain.FileEntry, error) {
	contents := make(map[string]domain.FileEntry)

	err := filepath.WalkDir(
		opts.sourceDir,
		collectModuleWalkFunc(opts, contents),
	)
	if err != nil {
		return nil, fmt.Errorf("walk module directory %q: %w", opts.sourceDir, err)
	}

	return contents, nil
}

func setLockConfiguration(lock *syncLock, cfg *config.Config) {
	lock.Configuration.TargetFolder = cfg.TargetFolder
	lock.Configuration.Tasks = append([]string{}, cfg.Tasks...)
	lock.Configuration.NodePackageManager = string(cfg.NodePackageManager)
	lock.Configuration.NodeVersionManager = string(cfg.NodeVersionManager)
	lock.Configuration.IncludesDoc = cfg.IncludesDoc
	lock.Configuration.SyncRoot = cfg.SyncRoot
}

func setLockResolvedModules(lock *syncLock, syncInput *domain.SyncInput) {
	lock.Requested = orderedRequested(syncInput.Requested)
	lock.Dependencies = append([]moduleRecord{}, syncInput.Dependencies...)
}

func setLockSource(lock *syncLock, syncInput *domain.SyncInput) {
	lock.Source.Repository = config.StoreRepository
	lock.Source.RequestedVersion = syncInput.Config.StoreVersion
	lock.Source.SourceRef = syncInput.Snapshot.SourceRef()
	lock.Source.ResolvedCommit = syncInput.Snapshot.ResolvedCommit()
	lock.Source.DefaultBranch = syncInput.Snapshot.DefaultBranch()
}

func shouldSkipModuleFile(rel string, policy docPolicy) bool {
	if policy == docPolicySkip && pathutil.IsDocPath(rel) {
		return true
	}

	return pathutil.IsTestPath(rel) || pathutil.IsModuleMetadataPath(rel)
}

func sortManagedFiles(planned []managedFile) {
	slices.SortFunc(planned, func(a, b managedFile) int {
		if a.Path == b.Path {
			return strings.Compare(a.SourceModule, b.SourceModule)
		}

		return strings.Compare(a.Path, b.Path)
	})
}

// sortedModules merges the requested and dependency modules into a single
// source-module-ordered slice so planning is deterministic.
func sortedModules(syncInput *domain.SyncInput) []moduleRecord {
	allModules := make(
		[]moduleRecord,
		consts.IndexZero,
		len(syncInput.Requested)+len(syncInput.Dependencies),
	)

	for key := range syncInput.Requested {
		allModules = append(allModules, syncInput.Requested[key])
	}

	allModules = append(allModules, syncInput.Dependencies...)
	slices.SortFunc(allModules, func(a, b moduleRecord) int {
		return strings.Compare(a.SourceModule, b.SourceModule)
	})

	return allModules
}

func storeCollectedModuleFile(args *moduleCollectArgs, rel string) error {
	entry, err := buildFileEntry(&fileEntryArgs{
		ops:          args.ops,
		sourceDir:    args.sourceDir,
		fromDest:     args.fromDest,
		rel:          rel,
		absPath:      args.absPath,
		entry:        args.entry,
		sourceToDest: args.sourceToDest,
	})
	if err != nil {
		return fmt.Errorf("build file entry: %w", err)
	}

	args.contents[rel] = entry

	return nil
}

func unmanagedDestinationError(mod *moduleRecord) domain.SyncError {
	return domain.SyncError(fmt.Sprintf(
		`Cannot copy source module %q to %q: the destination exists but is not managed by TaskOtter.`,
		mod.SourceModule,
		mod.Path,
	))
}

func updateRootTaskfile(input *updateRootArgs) (root []byte, tasks []generatedRootTask, err error) {
	ops := input.args.syncInput.TaskfileOps

	if ops == nil {
		return nil, nil, errTaskfileOpsNotConfigured
	}

	root, err = ops.UpdateRootTaskfile(input.args.rootBytes, rootUpdateInputFrom(input))
	if err != nil {
		return nil, nil, fmt.Errorf("update root Taskfile.yml: %w", err)
	}

	return root, input.generatedRootTasks, nil
}

func rootUpdateInputFrom(input *updateRootArgs) *rootUpdateInput {
	cfg := input.args.syncInput.Config

	return &rootUpdateInput{
		Tasks:            cfg.Tasks,
		TargetFolder:     cfg.TargetFolder,
		RootTaskfileDir:  path.Dir(cfg.RootTaskfile),
		DestByTask:       input.args.syncInput.DestByTask,
		ManagedTasks:     input.managedTasks,
		ModuleTaskfiles:  input.moduleTaskfiles,
		GeneratedTasks:   input.generatedRootTasks,
		ManagedRootTasks: input.managedRootTasks,
	}
}

func validateDestination(destDirAbs string, mod *moduleRecord, oldLock *syncLock) error {
	info, err := os.Stat(destDirAbs)

	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("stat destination %q: %w", mod.Path, err)
	}

	err = validateExistingDestination(info, mod, oldLock)
	if err != nil {
		return fmt.Errorf("validate existing destination: %w", err)
	}

	return nil
}

func validateExistingDestination(info os.FileInfo, mod *moduleRecord, oldLock *syncLock) error {
	if !info.IsDir() {
		return domain.SyncError(
			fmt.Sprintf("destination %q exists and is not a directory", mod.Path),
		)
	}

	if isDestinationManaged(oldLock, mod) {
		return nil
	}

	return unmanagedDestinationError(mod)
}

func validateModuleDestination(args *modulePlanArgs) (string, error) {
	destDirRel := pathutil.JoinRelative(
		args.syncInput.Config.TargetFolder,
		args.mod.DestinationModule,
	)
	destDirAbs := pathutil.WorkspacePath(args.syncInput.Config.Workspace, destDirRel)

	err := validateDestination(destDirAbs, args.mod, args.oldLock)
	if err != nil {
		return consts.Empty, fmt.Errorf("validate destination: %w", err)
	}

	return destDirRel, nil
}
