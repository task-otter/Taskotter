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
	"github.com/task-otter/Taskotter/internal/pathutil"
	"github.com/task-otter/Taskotter/internal/taskfile"
	"github.com/task-otter/Taskotter/internal/yamlfmt"
)

func mustMarshalMetadata(meta Metadata) []byte {
	data, err := yamlfmt.Marshal(meta)
	if err != nil {
		return nil
	}

	return data
}

// BuildPlan computes the sync diff and generated artifacts for syncInput.
func BuildPlan(syncInput SyncInput) (*Plan, error) {
	workspace := syncInput.Config.Workspace

	oldLock, _, oldTarget, err := loadPreviousState(workspace, syncInput.Config)
	if err != nil {
		return nil, err
	}

	plannedFiles, moduleContents, err := planManagedFiles(syncInput, oldLock)
	if err != nil {
		return nil, err
	}

	rootBytes, rootExisted, newRoot, generatedRootTasks, err := planRootTaskfile(
		syncInput,
		oldLock,
		moduleContents,
	)
	if err != nil {
		return nil, err
	}

	lock := buildLock(syncInput, plannedFiles, generatedRootTasks)
	meta := Metadata{
		TargetFolder:      syncInput.Config.TargetFolder,
		LockFile:          syncInput.Config.LockFilePath(),
		ConfigurationHash: syncInput.Config.ConfigurationHash,
	}

	plan := &Plan{
		Requested:        syncInput.Requested,
		Dependencies:     syncInput.Dependencies,
		ManagedFiles:     plannedFiles,
		ModuleContents:   moduleContents,
		RootTaskfile:     newRoot,
		RootTaskfilePath: syncInput.Config.RootTaskfile,
		Lock:             lock,
		Metadata:         meta,
		Added:            nil,
		Updated:          nil,
		Removed:          nil,
		Changed:          false,
		OldLock:          oldLock,
		OldTargetFolder:  oldTarget,
		StagePaths:       nil,
		copyFileTo:       nil,
	}

	return finalizePlanDiff(
		plan,
		workspace,
		rootBytes,
		rootExisted,
		syncInput.Config.SyncRoot,
		meta,
		syncInput.Config.MetadataPath(),
	)
}

func planRootTaskfile(
	syncInput SyncInput,
	oldLock *LockFile,
	moduleContents map[string]map[string]FileEntry,
) ([]byte, bool, []byte, []taskfile.GeneratedRootTask, error) {
	if !syncInput.Config.SyncRoot {
		return nil, false, nil, nil, nil
	}

	rootBytes, rootExisted, err := readRootTaskfile(
		syncInput.Config.Workspace,
		syncInput.Config.RootTaskfile,
	)
	if err != nil {
		return nil, false, nil, nil, err
	}

	newRoot, generatedRootTasks, err := buildRootTaskfile(
		syncInput,
		oldLock,
		moduleContents,
		rootBytes,
	)
	if err != nil {
		return nil, false, nil, nil, err
	}

	return rootBytes, rootExisted, newRoot, generatedRootTasks, nil
}

func readRootTaskfile(workspace, rootPath string) ([]byte, bool, error) {
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
	syncInput SyncInput,
	oldLock *LockFile,
	moduleContents map[string]map[string]FileEntry,
	rootBytes []byte,
) ([]byte, []taskfile.GeneratedRootTask, error) {
	managedTasks := syncInput.Config.Tasks
	managedRootTasks := []string(nil)

	if oldLock != nil {
		managedTasks = oldLock.Configuration.Tasks
		managedRootTasks = oldLock.GeneratedRootTasks
	}

	moduleTaskfiles := make(map[string][]byte, len(syncInput.Requested))

	for task, rec := range syncInput.Requested {
		files, ok := moduleContents[rec.SourceModule]

		if !ok {
			continue
		}

		if entry, ok := files[rootTaskfileName]; ok {
			moduleTaskfiles[task] = entry.Data
		}
	}

	storeMetadata, err := loadStoreTaskMetadata(syncInput.Snapshot)
	if err != nil {
		return nil, nil, err
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

func finalizePlanDiff(
	plan *Plan,
	workspace string,
	rootBytes []byte,
	rootExisted bool,
	syncRoot bool,
	meta Metadata,
	metadataPath string,
) (*Plan, error) {
	oldRootForDiff := rootBytes

	if !rootExisted {
		oldRootForDiff = nil
	}

	added, updated, removed, err := diffFiles(
		plan,
		workspace,
		oldRootForDiff,
		syncRoot,
		metadataPath,
		mustMarshalMetadata(meta),
	)
	if err != nil {
		return nil, err
	}

	plan.Added = added
	plan.Updated = updated
	plan.Removed = removed
	plan.Changed = len(added) > 0 || len(updated) > 0 || len(removed) > 0
	plan.StagePaths = buildStagePaths(plan, workspace, metadataPath, syncRoot)

	return plan, nil
}

// sortedModules merges the requested and dependency modules into a single
// source-module-ordered slice so planning is deterministic.
func sortedModules(syncInput SyncInput) []ModuleRecord {
	allModules := make([]ModuleRecord, 0, len(syncInput.Requested)+len(syncInput.Dependencies))

	for _, rec := range syncInput.Requested {
		allModules = append(allModules, rec)
	}

	allModules = append(allModules, syncInput.Dependencies...)
	sort.Slice(allModules, func(i, j int) bool {
		return allModules[i].SourceModule < allModules[j].SourceModule
	})

	return allModules
}

func planManagedFiles(
	syncInput SyncInput,
	oldLock *LockFile,
) ([]ManagedFile, map[string]map[string]FileEntry, error) {
	allModules := sortedModules(syncInput)
	moduleContents := make(map[string]map[string]FileEntry)

	var planned []ManagedFile

	for _, mod := range allModules {
		sourceDir := syncInput.Snapshot.ModuleDir(mod.SourceModule)

		_, err := os.Stat(sourceDir)
		if err != nil {
			return nil, nil, &SyncError{
				Message: fmt.Sprintf(
					"source module directory %q does not exist",
					mod.SourceModule,
				),
			}
		}

		destDirRel := pathutil.JoinRelative(syncInput.Config.TargetFolder, mod.DestinationModule)
		destDirAbs := pathutil.WorkspacePath(syncInput.Config.Workspace, destDirRel)

		err = validateDestination(destDirAbs, mod, oldLock)
		if err != nil {
			return nil, nil, err
		}

		contents, err := CollectModuleFiles(
			sourceDir,
			syncInput.Config.IncludesDoc,
			syncInput.SourceToDest,
		)
		if err != nil {
			return nil, nil, err
		}

		moduleContents[mod.SourceModule] = contents
		planned = appendManagedFiles(planned, mod, destDirRel, contents)
	}

	sort.Slice(planned, func(i, j int) bool {
		if planned[i].Path == planned[j].Path {
			return planned[i].SourceModule < planned[j].SourceModule
		}

		return planned[i].Path < planned[j].Path
	})

	return planned, moduleContents, nil
}

func appendManagedFiles(
	planned []ManagedFile,
	mod ModuleRecord,
	destDirRel string,
	contents map[string]FileEntry,
) []ManagedFile {
	for rel, entry := range contents {
		sum := sha256.Sum256(entry.Data)

		planned = append(planned, ManagedFile{
			SourceModule:      mod.SourceModule,
			DestinationModule: mod.DestinationModule,
			SourcePath:        pathutil.JoinRelative("taskfiles", mod.SourceModule, rel),
			Path:              pathutil.JoinRelative(destDirRel, rel),
			SHA256:            hex.EncodeToString(sum[:]),
		})
	}

	return planned
}

func validateDestination(destDirAbs string, mod ModuleRecord, oldLock *LockFile) error {
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

	if oldLock == nil {
		return unmanagedDestinationError(mod)
	}

	for _, managed := range oldLock.ManagedFiles {
		if managed.DestinationModule == mod.DestinationModule {
			return nil
		}
	}

	return unmanagedDestinationError(mod)
}

func unmanagedDestinationError(mod ModuleRecord) *SyncError {
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

	err := filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", path, walkErr)
		}

		if entry.IsDir() {
			return nil
		}

		return collectModuleFile(sourceDir, path, entry, includesDoc, sourceToDest, contents)
	})
	if err != nil {
		return nil, fmt.Errorf("walk module directory %q: %w", sourceDir, err)
	}

	return contents, nil
}

func collectModuleFile(
	sourceDir, path string,
	entry os.DirEntry,
	includesDoc bool,
	sourceToDest map[string]string,
	contents map[string]FileEntry,
) error {
	rel, err := filepath.Rel(sourceDir, path)
	if err != nil {
		return fmt.Errorf("rel path for %q: %w", path, err)
	}

	rel = filepath.ToSlash(rel)

	if !includesDoc && pathutil.IsDocPath(rel) {
		return nil
	}

	if pathutil.IsTestPath(rel) || pathutil.IsModuleMetadataPath(rel) {
		return nil
	}

	info, err := entry.Info()
	if err != nil {
		return fmt.Errorf("file info for %q: %w", path, err)
	}

	data, err := pathutil.ReadRelativeFile(sourceDir, rel)
	if err != nil {
		return fmt.Errorf("read module file %q: %w", path, err)
	}

	if rel == rootTaskfileName {
		data, err = taskfile.RewriteIncludes(data, sourceToDest)
		if err != nil {
			return fmt.Errorf("rewrite includes in %q: %w", path, err)
		}
	}

	contents[rel] = FileEntry{Data: data, Mode: preserveMode(info.Mode())}

	return nil
}

func buildLock(
	syncInput SyncInput,
	files []ManagedFile,
	generatedRootTasks []taskfile.GeneratedRootTask,
) LockFile {
	var lock LockFile

	lock.Source.Repository = config.StoreRepository
	lock.Source.RequestedVersion = syncInput.Config.StoreVersion
	lock.Source.SourceRef = syncInput.Snapshot.Ref.SourceRef
	lock.Source.ResolvedCommit = syncInput.Snapshot.Ref.ResolvedCommit
	lock.Source.DefaultBranch = syncInput.Snapshot.Ref.DefaultBranch
	lock.Configuration.TargetFolder = syncInput.Config.TargetFolder
	lock.Configuration.Tasks = append([]string{}, syncInput.Config.Tasks...)
	lock.Configuration.NodePackageManager = string(syncInput.Config.NodePackageManager)
	lock.Configuration.NodeVersionManager = string(syncInput.Config.NodeVersionManager)
	lock.Configuration.IncludesDoc = syncInput.Config.IncludesDoc
	lock.Configuration.SyncRoot = syncInput.Config.SyncRoot
	lock.ResolvedModules.Requested = orderedRequested(syncInput.Requested)
	lock.ResolvedModules.Dependencies = append([]ModuleRecord{}, syncInput.Dependencies...)
	lock.GeneratedRootTasks = generatedRootTaskNames(generatedRootTasks)
	lock.ManagedFiles = files

	return lock
}

func generatedRootTaskNames(generated []taskfile.GeneratedRootTask) []string {
	names := make([]string, 0, len(generated))

	for _, task := range generated {
		names = append(names, task.Name)
	}

	sort.Strings(names)

	return names
}
