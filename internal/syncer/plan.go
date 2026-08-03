// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
	"github.com/task-otter/Taskotter/internal/taskfile"
	"github.com/task-otter/Taskotter/internal/yamlfmt"
)

// planArtifacts bundles the intermediate outputs produced while planning managed
// files and the generated root Taskfile, before they're assembled into a Plan.
type planArtifacts struct {
	moduleContents     map[string]map[string]FileEntry
	plannedFiles       []ManagedFile
	rootBytes          []byte
	newRoot            []byte
	generatedRootTasks []taskfile.GeneratedRootTask
	rootExisted        bool
}

func mustMarshalMetadata(meta *Metadata) []byte {
	data, err := yamlfmt.Marshal(meta)
	if err != nil {
		return nil
	}

	return data
}

// BuildPlan computes the sync diff and generated artifacts for syncInput.
func BuildPlan(syncInput *SyncInput) (*Plan, error) {
	oldLock, _, oldTarget, err := loadPreviousState(syncInput.Config.Workspace, syncInput.Config)
	if err != nil {
		return nil, fmt.Errorf("load previous state: %w", err)
	}

	artifacts, err := planAllFiles(syncInput, oldLock)
	if err != nil {
		return nil, fmt.Errorf("plan all files: %w", err)
	}

	plan, meta := newPlanFromInputs(syncInput, &artifacts, oldLock, oldTarget)

	finalPlan, err := finalizePlanDiff(
		plan,
		syncInput.Config.Workspace,
		artifacts.rootBytes,
		artifacts.rootExisted,
		syncInput.Config.SyncRoot,
		meta,
		syncInput.Config.MetadataPath(),
	)
	if err != nil {
		return nil, fmt.Errorf("finalize plan diff: %w", err)
	}

	return finalPlan, nil
}

func planAllFiles(syncInput *SyncInput, oldLock *LockFile) (planArtifacts, error) {
	plannedFiles, moduleContents, err := planManagedFiles(syncInput, oldLock)
	if err != nil {
		return planArtifacts{}, fmt.Errorf("plan managed files: %w", err)
	}

	rootBytes, rootExisted, newRoot, generatedRootTasks, err := planRootTaskfile(
		syncInput,
		oldLock,
		moduleContents,
	)
	if err != nil {
		return planArtifacts{}, fmt.Errorf("plan root taskfile: %w", err)
	}

	return planArtifacts{
		plannedFiles:       plannedFiles,
		moduleContents:     moduleContents,
		rootBytes:          rootBytes,
		rootExisted:        rootExisted,
		newRoot:            newRoot,
		generatedRootTasks: generatedRootTasks,
	}, nil
}

func newPlanFromInputs(
	syncInput *SyncInput,
	artifacts *planArtifacts,
	oldLock *LockFile,
	oldTarget string,
) (*Plan, *Metadata) {
	lock := buildLock(syncInput, artifacts.plannedFiles, artifacts.generatedRootTasks)
	meta := &Metadata{
		TargetFolder:      syncInput.Config.TargetFolder,
		LockFile:          syncInput.Config.LockFilePath(),
		ConfigurationHash: syncInput.Config.ConfigurationHash,
	}

	plan := &Plan{
		Requested:        syncInput.Requested,
		Dependencies:     syncInput.Dependencies,
		ManagedFiles:     artifacts.plannedFiles,
		ModuleContents:   artifacts.moduleContents,
		RootTaskfile:     artifacts.newRoot,
		RootTaskfilePath: syncInput.Config.RootTaskfile,
		Lock:             lock,
		Metadata:         *meta,
		Added:            nil,
		Updated:          nil,
		Removed:          nil,
		Changed:          false,
		OldLock:          oldLock,
		OldTargetFolder:  oldTarget,
		StagePaths:       nil,
		copyFileTo:       nil,
	}

	return plan, meta
}

func planRootTaskfile(
	syncInput *SyncInput,
	oldLock *LockFile,
	moduleContents map[string]map[string]FileEntry,
) (rootBytes []byte, rootExisted bool, newRoot []byte, generatedRootTasks []taskfile.GeneratedRootTask, err error) {
	if !syncInput.Config.SyncRoot {
		return nil, false, nil, nil, nil
	}

	rootBytes, rootExisted, err = readRootTaskfile(
		syncInput.Config.Workspace,
		syncInput.Config.RootTaskfile,
	)
	if err != nil {
		return nil, false, nil, nil, fmt.Errorf("read root taskfile: %w", err)
	}

	newRoot, generatedRootTasks, err = buildRootTaskfile(
		syncInput,
		oldLock,
		moduleContents,
		rootBytes,
	)
	if err != nil {
		return nil, false, nil, nil, fmt.Errorf("build root taskfile: %w", err)
	}

	return rootBytes, rootExisted, newRoot, generatedRootTasks, nil
}

func readRootTaskfile(workspace, rootPath string) (data []byte, existed bool, err error) {
	rootBytes, err := pathutil.ReadRelativeFile(workspace, rootPath)
	if err == nil {
		return rootBytes, true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return taskfile.NewRootTemplate(), false, nil
	}

	return nil, false, fmt.Errorf("read root Taskfile.yml: %w", err)
}

func buildRootTaskfile(
	syncInput *SyncInput,
	oldLock *LockFile,
	moduleContents map[string]map[string]FileEntry,
	rootBytes []byte,
) ([]byte, []taskfile.GeneratedRootTask, error) {
	managedTasks, managedRootTasks := resolveManagedTasks(syncInput, oldLock)
	moduleTaskfiles := collectModuleRootTaskfiles(syncInput.Requested, moduleContents)

	storeMetadata, err := loadStoreTaskMetadata(syncInput.Snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("load store task metadata: %w", err)
	}

	generatedRootTasks := buildGeneratedRootTasks(
		syncInput.Config.Tasks,
		syncInput.Requested,
		storeMetadata,
	)

	newRoot, err := taskfile.UpdateRootTaskfile(rootBytes, taskfile.RootUpdateInput{
		Tasks:            syncInput.Config.Tasks,
		TargetFolder:     syncInput.Config.TargetFolder,
		RootTaskfileDir:  path.Dir(syncInput.Config.RootTaskfile),
		DestByTask:       syncInput.DestByTask,
		ManagedTasks:     managedTasks,
		ModuleTaskfiles:  moduleTaskfiles,
		GeneratedTasks:   generatedRootTasks,
		ManagedRootTasks: managedRootTasks,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("update root Taskfile.yml: %w", err)
	}

	return newRoot, generatedRootTasks, nil
}

func resolveManagedTasks(
	syncInput *SyncInput,
	oldLock *LockFile,
) (managedTasks, managedRootTasks []string) {
	if oldLock != nil {
		return oldLock.Configuration.Tasks, oldLock.GeneratedRootTasks
	}

	return syncInput.Config.Tasks, nil
}

func collectModuleRootTaskfiles(
	requested map[string]ModuleRecord,
	moduleContents map[string]map[string]FileEntry,
) map[string][]byte {
	moduleTaskfiles := make(map[string][]byte, len(requested))

	for task := range requested {
		rec := requested[task]
		files, ok := moduleContents[rec.SourceModule]

		if !ok {
			continue
		}

		if entry, ok := files[rootTaskfileName]; ok {
			moduleTaskfiles[task] = entry.Data
		}
	}

	return moduleTaskfiles
}

func finalizePlanDiff(
	plan *Plan,
	workspace string,
	rootBytes []byte,
	rootExisted bool,
	syncRoot bool,
	meta *Metadata,
	metadataPath string,
) (*Plan, error) {
	added, updated, removed, err := diffFiles(
		plan,
		workspace,
		oldRootForDiffing(rootBytes, rootExisted),
		syncRoot,
		metadataPath,
		mustMarshalMetadata(meta),
	)
	if err != nil {
		return nil, fmt.Errorf("diff files: %w", err)
	}

	applyDiffResults(plan, added, updated, removed, workspace, metadataPath, syncRoot)

	return plan, nil
}

func oldRootForDiffing(rootBytes []byte, rootExisted bool) []byte {
	if !rootExisted {
		return nil
	}

	return rootBytes
}

func applyDiffResults(
	plan *Plan,
	added, updated, removed []string,
	workspace, metadataPath string,
	syncRoot bool,
) {
	plan.Added = added
	plan.Updated = updated
	plan.Removed = removed
	plan.Changed = anyChanges(added, updated, removed)
	plan.StagePaths = buildStagePaths(plan, workspace, metadataPath, syncRoot)
}

func anyChanges(added, updated, removed []string) bool {
	return len(added) > consts.IndexZero || len(updated) > consts.IndexZero ||
		len(removed) > consts.IndexZero
}

// sortedModules merges the requested and dependency modules into a single
// source-module-ordered slice so planning is deterministic.
func sortedModules(syncInput *SyncInput) []ModuleRecord {
	allModules := make(
		[]ModuleRecord,
		consts.IndexZero,
		len(syncInput.Requested)+len(syncInput.Dependencies),
	)

	for key := range syncInput.Requested {
		allModules = append(allModules, syncInput.Requested[key])
	}

	allModules = append(allModules, syncInput.Dependencies...)
	sort.Slice(allModules, func(i, j int) bool {
		return allModules[i].SourceModule < allModules[j].SourceModule
	})

	return allModules
}

func planManagedFiles(
	syncInput *SyncInput,
	oldLock *LockFile,
) ([]ManagedFile, map[string]map[string]FileEntry, error) {
	allModules := sortedModules(syncInput)
	moduleContents := make(map[string]map[string]FileEntry)

	var planned []ManagedFile

	for i := range allModules {
		mod := &allModules[i]

		var err error

		planned, err = accumulateModulePlan(syncInput, mod, oldLock, moduleContents, planned)
		if err != nil {
			return nil, nil, fmt.Errorf("accumulate module plan for %q: %w", mod.SourceModule, err)
		}
	}

	sortManagedFiles(planned)

	return planned, moduleContents, nil
}

func accumulateModulePlan(
	syncInput *SyncInput,
	mod *ModuleRecord,
	oldLock *LockFile,
	moduleContents map[string]map[string]FileEntry,
	planned []ManagedFile,
) ([]ManagedFile, error) {
	contents, managed, err := planModuleFiles(syncInput, mod, oldLock)
	if err != nil {
		return nil, fmt.Errorf("plan module files: %w", err)
	}

	moduleContents[mod.SourceModule] = contents

	return append(planned, managed...), nil
}

func planModuleFiles(
	syncInput *SyncInput,
	mod *ModuleRecord,
	oldLock *LockFile,
) (map[string]FileEntry, []ManagedFile, error) {
	sourceDir := syncInput.Snapshot.ModuleDir(mod.SourceModule)

	err := ensureSourceDirExists(sourceDir, mod)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure source dir exists: %w", err)
	}

	destDirRel, err := validateModuleDestination(syncInput, mod, oldLock)
	if err != nil {
		return nil, nil, fmt.Errorf("validate module destination: %w", err)
	}

	contents, managed, err := collectAndTrackModuleFiles(syncInput, mod, sourceDir, destDirRel)
	if err != nil {
		return nil, nil, fmt.Errorf("collect and track module files: %w", err)
	}

	return contents, managed, nil
}

func ensureSourceDirExists(sourceDir string, mod *ModuleRecord) error {
	_, err := os.Stat(sourceDir)
	if err != nil {
		return &SyncError{
			Message: fmt.Sprintf("source module directory %q does not exist", mod.SourceModule),
		}
	}

	return nil
}

func validateModuleDestination(
	syncInput *SyncInput,
	mod *ModuleRecord,
	oldLock *LockFile,
) (string, error) {
	destDirRel := pathutil.JoinRelative(syncInput.Config.TargetFolder, mod.DestinationModule)
	destDirAbs := pathutil.WorkspacePath(syncInput.Config.Workspace, destDirRel)

	err := validateDestination(destDirAbs, mod, oldLock)
	if err != nil {
		return consts.Empty, fmt.Errorf("validate destination: %w", err)
	}

	return destDirRel, nil
}

func collectAndTrackModuleFiles(
	syncInput *SyncInput,
	mod *ModuleRecord,
	sourceDir, destDirRel string,
) (map[string]FileEntry, []ManagedFile, error) {
	contents, err := CollectModuleFiles(
		sourceDir,
		syncInput.Config.IncludesDoc,
		syncInput.SourceToDest,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("collect module files: %w", err)
	}

	managed := appendManagedFiles(nil, mod, destDirRel, contents)

	return contents, managed, nil
}

func sortManagedFiles(planned []ManagedFile) {
	sort.Slice(planned, func(i, j int) bool {
		if planned[i].Path == planned[j].Path {
			return planned[i].SourceModule < planned[j].SourceModule
		}

		return planned[i].Path < planned[j].Path
	})
}

func appendManagedFiles(
	planned []ManagedFile,
	mod *ModuleRecord,
	destDirRel string,
	contents map[string]FileEntry,
) []ManagedFile {
	for rel := range contents {
		entry := contents[rel]
		sum := sha256.Sum256(entry.Data)

		planned = append(planned, ManagedFile{
			SourceModule:      mod.SourceModule,
			DestinationModule: mod.DestinationModule,
			SourcePath:        pathutil.JoinRelative(taskfilesDirName, mod.SourceModule, rel),
			Path:              pathutil.JoinRelative(destDirRel, rel),
			SHA256:            hex.EncodeToString(sum[:]),
		})
	}

	return planned
}

func validateDestination(destDirAbs string, mod *ModuleRecord, oldLock *LockFile) error {
	info, err := os.Stat(destDirAbs)

	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("stat destination %q: %w", mod.Path, err)
	}

	if !info.IsDir() {
		return &SyncError{
			Message: fmt.Sprintf("destination %q exists and is not a directory", mod.Path),
		}
	}

	if isDestinationManaged(oldLock, mod) {
		return nil
	}

	return unmanagedDestinationError(mod)
}

func isDestinationManaged(oldLock *LockFile, mod *ModuleRecord) bool {
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

func unmanagedDestinationError(mod *ModuleRecord) *SyncError {
	return &SyncError{
		Message: fmt.Sprintf(
			`Cannot copy source module %q to %q: the destination exists but is not managed by TaskOtter.`,
			mod.SourceModule,
			mod.Path,
		),
	}
}

// CollectModuleFiles scans a module source directory and returns syncable file entries.
func CollectModuleFiles(
	sourceDir string,
	includesDoc bool,
	sourceToDest map[string]string,
) (map[string]FileEntry, error) {
	contents := make(map[string]FileEntry)

	err := filepath.WalkDir(
		sourceDir,
		func(absPath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf(errWalk, absPath, walkErr)
			}

			if entry.IsDir() {
				return nil
			}

			return collectModuleFile(sourceDir, absPath, entry, includesDoc, sourceToDest, contents)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("walk module directory %q: %w", sourceDir, err)
	}

	return contents, nil
}

func collectModuleFile(
	sourceDir, absPath string,
	entry os.DirEntry,
	includesDoc bool,
	sourceToDest map[string]string,
	contents map[string]FileEntry,
) error {
	rel, err := relSlashPath(sourceDir, absPath)
	if err != nil {
		return fmt.Errorf("rel slash path: %w", err)
	}

	if shouldSkipModuleFile(rel, includesDoc) {
		return nil
	}

	fileEntry, err := buildFileEntry(sourceDir, rel, absPath, entry, sourceToDest)
	if err != nil {
		return fmt.Errorf("build file entry: %w", err)
	}

	contents[rel] = fileEntry

	return nil
}

func relSlashPath(sourceDir, absPath string) (string, error) {
	rel, err := filepath.Rel(sourceDir, absPath)
	if err != nil {
		return consts.Empty, fmt.Errorf("rel path for %q: %w", absPath, err)
	}

	return filepath.ToSlash(rel), nil
}

func shouldSkipModuleFile(rel string, includesDoc bool) bool {
	if !includesDoc && pathutil.IsDocPath(rel) {
		return true
	}

	return pathutil.IsTestPath(rel) || pathutil.IsModuleMetadataPath(rel)
}

func buildFileEntry(
	sourceDir, rel, absPath string,
	entry os.DirEntry,
	sourceToDest map[string]string,
) (FileEntry, error) {
	info, err := entry.Info()
	if err != nil {
		return FileEntry{}, fmt.Errorf("file info for %q: %w", absPath, err)
	}

	data, err := readAndMaybeRewriteModuleFile(sourceDir, rel, absPath, sourceToDest)
	if err != nil {
		return FileEntry{}, fmt.Errorf("read/rewrite module file: %w", err)
	}

	return FileEntry{Data: data, Mode: preserveMode(info.Mode())}, nil
}

func readAndMaybeRewriteModuleFile(
	sourceDir, rel, absPath string,
	sourceToDest map[string]string,
) ([]byte, error) {
	data, err := pathutil.ReadRelativeFile(sourceDir, rel)
	if err != nil {
		return nil, fmt.Errorf("read module file %q: %w", absPath, err)
	}

	if rel != rootTaskfileName {
		return data, nil
	}

	rewritten, err := taskfile.RewriteIncludes(data, sourceToDest)
	if err != nil {
		return nil, fmt.Errorf("rewrite includes in %q: %w", absPath, err)
	}

	return rewritten, nil
}

func buildLock(
	syncInput *SyncInput,
	files []ManagedFile,
	generatedRootTasks []taskfile.GeneratedRootTask,
) LockFile {
	var lock LockFile

	setLockSource(&lock, syncInput)
	setLockConfiguration(&lock, syncInput.Config)
	setLockResolvedModules(&lock, syncInput)

	lock.GeneratedRootTasks = generatedRootTaskNames(generatedRootTasks)
	lock.ManagedFiles = files

	return lock
}

func setLockSource(lock *LockFile, syncInput *SyncInput) {
	lock.Source.Repository = config.StoreRepository
	lock.Source.RequestedVersion = syncInput.Config.StoreVersion
	lock.Source.SourceRef = syncInput.Snapshot.Ref.SourceRef
	lock.Source.ResolvedCommit = syncInput.Snapshot.Ref.ResolvedCommit
	lock.Source.DefaultBranch = syncInput.Snapshot.Ref.DefaultBranch
}

func setLockConfiguration(lock *LockFile, cfg *config.Config) {
	lock.Configuration.TargetFolder = cfg.TargetFolder
	lock.Configuration.Tasks = append([]string{}, cfg.Tasks...)
	lock.Configuration.NodePackageManager = string(cfg.NodePackageManager)
	lock.Configuration.NodeVersionManager = string(cfg.NodeVersionManager)
	lock.Configuration.IncludesDoc = cfg.IncludesDoc
	lock.Configuration.SyncRoot = cfg.SyncRoot
}

func setLockResolvedModules(lock *LockFile, syncInput *SyncInput) {
	lock.ResolvedModules.Requested = orderedRequested(syncInput.Requested)
	lock.ResolvedModules.Dependencies = append([]ModuleRecord{}, syncInput.Dependencies...)
}

func generatedRootTaskNames(generated []taskfile.GeneratedRootTask) []string {
	names := make([]string, consts.IndexZero, len(generated))

	for i := range generated {
		names = append(names, generated[i].Name)
	}

	sort.Strings(names)

	return names
}
