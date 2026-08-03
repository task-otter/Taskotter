// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package syncer plans and applies taskfile synchronization changes to the workspace.
package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
	"github.com/task-otter/Taskotter/internal/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

type stagedFile struct {
	finalRel string
	entry    FileEntry
}

const (
	lockFileName          = ".taskotter-lock.yml"
	legacyMetadataDirName = ".taskotter"
	legacyMetadataRelPath = ".taskotter/metadata.yml"
	errCreateStagingDir   = "create staging directory: %w"
)

func buildStagedFiles(plan *Plan, syncInput *SyncInput) ([]stagedFile, error) {
	staged := stageModuleFiles(plan, syncInput.Config.TargetFolder)

	if syncInput.Config.SyncRoot {
		staged = append(staged, stagedFile{
			finalRel: plan.RootTaskfilePath,
			entry:    FileEntry{Data: plan.RootTaskfile, Mode: fileModeRegular},
		})
	}

	lockAndMeta, err := stageLockAndMetadata(plan, syncInput.Config.MetadataPath())
	if err != nil {
		return nil, fmt.Errorf("stage lock and metadata: %w", err)
	}

	return append(staged, lockAndMeta...), nil
}

func stageModuleFiles(plan *Plan, targetFolder string) []stagedFile {
	var staged []stagedFile

	records := sortedModuleRecords(plan.Requested, plan.Dependencies)

	for i := range records {
		mod := &records[i]

		staged = append(
			staged,
			stageOneModule(mod, plan.ModuleContents[mod.SourceModule], targetFolder)...,
		)
	}

	return staged
}

func stageOneModule(
	mod *ModuleRecord,
	contents map[string]FileEntry,
	targetFolder string,
) []stagedFile {
	rels := make([]string, consts.IndexZero, len(contents))

	for rel := range contents {
		rels = append(rels, rel)
	}

	sort.Strings(rels)

	destDirRel := pathutil.JoinRelative(targetFolder, mod.DestinationModule)
	staged := make([]stagedFile, consts.IndexZero, len(rels))

	for i := range rels {
		rel := rels[i]
		finalRel := pathutil.JoinRelative(destDirRel, rel)

		staged = append(staged, stagedFile{finalRel: finalRel, entry: contents[rel]})
	}

	return staged
}

func stageLockAndMetadata(plan *Plan, metadataPath string) ([]stagedFile, error) {
	lockBytes, err := yamlfmt.Marshal(plan.Lock)
	if err != nil {
		return nil, fmt.Errorf("marshal lock file: %w", err)
	}

	metaBytes, err := yamlfmt.Marshal(plan.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	return []stagedFile{
		{
			finalRel: plan.Metadata.LockFile,
			entry:    FileEntry{Data: lockBytes, Mode: fileModeRegular},
		},
		{
			finalRel: metadataPath,
			entry:    FileEntry{Data: metaBytes, Mode: fileModeRegular},
		},
	}, nil
}

// ApplyPlan writes planned files atomically and removes obsolete managed paths.
func ApplyPlan(plan *Plan, syncInput *SyncInput) (err error) {
	workspace := syncInput.Config.Workspace

	staged, copyFile, stagingRoot, err := prepareStaging(plan, syncInput, workspace)
	if err != nil {
		return fmt.Errorf("prepare staging: %w", err)
	}

	defer func() {
		removeErr := os.RemoveAll(stagingRoot)

		if removeErr != nil && err == nil {
			err = fmt.Errorf("clean up staging directory %q: %w", stagingRoot, removeErr)
		}
	}()

	err = validateAndWriteStaged(staged, plan.RootTaskfilePath, workspace, copyFile)
	if err != nil {
		return fmt.Errorf("validate and write staged files: %w", err)
	}

	err = cleanupAfterApply(plan, workspace, syncInput.Config.MetadataPath())
	if err != nil {
		return fmt.Errorf("clean up after apply: %w", err)
	}

	return nil
}

func prepareStaging(
	plan *Plan,
	syncInput *SyncInput,
	workspace string,
) (staged []stagedFile, copyFile func(string, FileEntry) error, stagingRoot string, err error) {
	staged, err = buildStagedFiles(plan, syncInput)
	if err != nil {
		return nil, nil, consts.Empty, fmt.Errorf("build staged files: %w", err)
	}

	copyFile = resolveCopyHook(plan)

	stagingRoot, err = stagePlanFiles(staged, workspace, syncInput.Config.TargetFolder, copyFile)
	if err != nil {
		return nil, nil, consts.Empty, fmt.Errorf("stage plan files: %w", err)
	}

	return staged, copyFile, stagingRoot, nil
}

func resolveCopyHook(plan *Plan) func(string, FileEntry) error {
	if plan.copyFileTo != nil {
		return plan.copyFileTo
	}

	return copyFileTo
}

func validateAndWriteStaged(
	staged []stagedFile,
	rootPath, workspace string,
	copyFile func(string, FileEntry) error,
) error {
	err := validateGeneratedYAML(staged, rootPath)
	if err != nil {
		return fmt.Errorf("validate generated yaml: %w", err)
	}

	err = writeStagedFiles(staged, workspace, copyFile)
	if err != nil {
		return fmt.Errorf("write staged files: %w", err)
	}

	return nil
}

func cleanupAfterApply(plan *Plan, workspace, metadataPath string) error {
	err := removeObsolete(plan, workspace)
	if err != nil {
		return fmt.Errorf("remove obsolete files: %w", err)
	}

	err = cleanupLegacyMetadata(workspace, metadataPath)
	if err != nil {
		return fmt.Errorf("clean up legacy metadata: %w", err)
	}

	return nil
}

func stagePlanFiles(
	staged []stagedFile,
	workspace, targetFolder string,
	copyFile func(string, FileEntry) error,
) (string, error) {
	stagingRoot, err := prepareStagingRoot(workspace, targetFolder)
	if err != nil {
		return consts.Empty, fmt.Errorf("prepare staging root: %w", err)
	}

	err = copyStagedFiles(stagingRoot, staged, copyFile)
	if err != nil {
		copyErr := fmt.Errorf("copy staged files: %w", err)

		removeErr := os.RemoveAll(stagingRoot)
		if removeErr != nil {
			return consts.Empty, errors.Join(
				copyErr,
				fmt.Errorf("clean up staging directory %q: %w", stagingRoot, removeErr),
			)
		}

		return consts.Empty, copyErr
	}

	return stagingRoot, nil
}

func prepareStagingRoot(workspace, targetFolder string) (string, error) {
	stagingParent := pathutil.WorkspacePath(
		workspace,
		pathutil.JoinRelative(targetFolder, ".taskotter/staging"),
	)

	err := os.MkdirAll(stagingParent, dirModePerm)
	if err != nil {
		return consts.Empty, fmt.Errorf(errCreateStagingDir, err)
	}

	stagingRoot, err := os.MkdirTemp(stagingParent, "apply-*")
	if err != nil {
		return consts.Empty, fmt.Errorf(errCreateStagingDir, err)
	}

	return stagingRoot, nil
}

func copyStagedFiles(
	stagingRoot string,
	staged []stagedFile,
	copyFile func(string, FileEntry) error,
) error {
	for i := range staged {
		stagedEntry := &staged[i]
		stagePath := filepath.Join(stagingRoot, filepath.FromSlash(stagedEntry.finalRel))

		err := copyFile(stagePath, stagedEntry.entry)
		if err != nil {
			return fmt.Errorf("stage %q: %w", stagedEntry.finalRel, err)
		}
	}

	return nil
}

func writeStagedFiles(
	staged []stagedFile,
	workspace string,
	copyFile func(string, FileEntry) error,
) error {
	for i := range staged {
		stagedEntry := &staged[i]
		finalPath := pathutil.WorkspacePath(workspace, stagedEntry.finalRel)

		err := os.MkdirAll(filepath.Dir(finalPath), dirModePerm)
		if err != nil {
			return fmt.Errorf("prepare %q: %w", stagedEntry.finalRel, err)
		}

		err = copyFile(finalPath, stagedEntry.entry)
		if err != nil {
			return fmt.Errorf("write %q: %w", stagedEntry.finalRel, err)
		}
	}

	return nil
}

func validateGeneratedYAML(staged []stagedFile, rootPath string) error {
	for i := range staged {
		stagedEntry := &staged[i]

		err := validateStagedYAML(stagedEntry, rootPath)
		if err != nil {
			return fmt.Errorf("validate staged yaml %q: %w", stagedEntry.finalRel, err)
		}
	}

	return nil
}

func validateStagedYAML(stagedEntry *stagedFile, rootPath string) error {
	var err error

	switch {
	case stagedEntry.finalRel == rootPath:
		err = validateRootTaskfileYAML(stagedEntry.entry.Data)
	case filepath.Base(stagedEntry.finalRel) == lockFileName:
		err = validateLockFileYAML(stagedEntry.entry.Data)
	case filepath.Base(stagedEntry.finalRel) == storeMetadataFileName:
		err = validateMetadataYAML(stagedEntry.entry.Data)
	default:
		return nil
	}

	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	return nil
}

func validateRootTaskfileYAML(data []byte) error {
	var node yaml.Node

	err := yaml.Unmarshal(data, &node)
	if err != nil {
		return fmt.Errorf("validate root Taskfile.yml: %w", err)
	}

	return nil
}

func validateLockFileYAML(data []byte) error {
	var lock LockFile

	err := yaml.Unmarshal(data, &lock)
	if err != nil {
		return fmt.Errorf("validate lock file: %w", err)
	}

	return nil
}

func validateMetadataYAML(data []byte) error {
	var meta Metadata

	err := yaml.Unmarshal(data, &meta)
	if err != nil {
		return fmt.Errorf("validate metadata: %w", err)
	}

	return nil
}

func removeObsolete(plan *Plan, workspace string) error {
	currentPaths := buildCurrentPathSet(plan)

	if plan.OldLock == nil {
		return nil
	}

	err := removeStaleManagedFiles(plan.OldLock, currentPaths, workspace)
	if err != nil {
		return fmt.Errorf("remove stale managed files: %w", err)
	}

	err = cleanupOldTarget(plan, workspace)
	if err != nil {
		return fmt.Errorf("clean up old target: %w", err)
	}

	return nil
}

func buildCurrentPathSet(plan *Plan) map[string]struct{} {
	currentPaths := make(map[string]struct{}, len(plan.ManagedFiles))

	for i := range plan.ManagedFiles {
		currentPaths[plan.ManagedFiles[i].Path] = struct{}{}
	}

	return currentPaths
}

func removeStaleManagedFiles(
	oldLock *LockFile,
	currentPaths map[string]struct{},
	workspace string,
) error {
	for i := range oldLock.ManagedFiles {
		old := &oldLock.ManagedFiles[i]

		if _, ok := currentPaths[old.Path]; ok {
			continue
		}

		err := removeObsoleteFile(workspace, old.Path)
		if err != nil {
			return fmt.Errorf("remove obsolete file %q: %w", old.Path, err)
		}
	}

	return nil
}

func removeObsoleteFile(workspace, path string) error {
	abs := pathutil.WorkspacePath(workspace, path)

	err := os.Remove(abs)

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove obsolete file %q: %w", path, err)
	}

	return nil
}

func cleanupOldTarget(plan *Plan, workspace string) error {
	if oldTargetUnchanged(plan) {
		return nil
	}

	err := removeOldTargetFiles(plan, workspace)
	if err != nil {
		return fmt.Errorf("remove old target files: %w", err)
	}

	err = removeIfExists(
		workspace,
		pathutil.JoinRelative(plan.OldTargetFolder, lockFileName),
	)
	if err != nil {
		return fmt.Errorf("remove old lock file: %w", err)
	}

	err = removeOldTargetMetadata(plan, workspace)
	if err != nil {
		return fmt.Errorf("remove old target metadata: %w", err)
	}

	return nil
}

func oldTargetUnchanged(plan *Plan) bool {
	return plan.OldLock == nil || plan.OldTargetFolder == "" ||
		plan.OldTargetFolder == plan.Metadata.TargetFolder
}

func removeOldTargetFiles(plan *Plan, workspace string) error {
	for i := range plan.OldLock.ManagedFiles {
		old := &plan.OldLock.ManagedFiles[i]

		if !pathutil.HasFolderPrefix(old.Path, plan.OldTargetFolder) {
			continue
		}

		err := removeIfExists(workspace, old.Path)
		if err != nil {
			return fmt.Errorf("remove old target file %q: %w", old.Path, err)
		}
	}

	return nil
}

func removeOldTargetMetadata(plan *Plan, workspace string) error {
	oldMetadataRel := pathutil.JoinRelative(plan.OldTargetFolder, legacyMetadataRelPath)

	err := removeIfExists(workspace, oldMetadataRel)
	if err != nil {
		return fmt.Errorf("remove old metadata file: %w", err)
	}

	oldMetadata := pathutil.WorkspacePath(workspace, oldMetadataRel)

	err = removeDirIfEmpty(filepath.Dir(oldMetadata), "remove old metadata directory")
	if err != nil {
		return fmt.Errorf("clean up old metadata directory: %w", err)
	}

	return nil
}

func removeIfExists(workspace, rel string) error {
	err := os.Remove(pathutil.WorkspacePath(workspace, rel))

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %q: %w", rel, err)
	}

	return nil
}

func cleanupLegacyMetadata(workspace, metadataPath string) error {
	if metadataPath == config.LegacyMetadataPath {
		return nil
	}

	legacy := pathutil.WorkspacePath(workspace, config.LegacyMetadataPath)

	err := os.Remove(legacy)

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy metadata: %w", err)
	}

	legacyDir := pathutil.WorkspacePath(workspace, legacyMetadataDirName)

	err = removeDirIfEmpty(legacyDir, "remove legacy metadata directory")
	if err != nil {
		return fmt.Errorf("clean up legacy metadata directory: %w", err)
	}

	return nil
}

func removeDirIfEmpty(dir, context string) error {
	err := os.Remove(dir)

	if err != nil && !os.IsNotExist(err) && !errorsIsDirectoryNotEmpty(err) {
		return fmt.Errorf("%s: %w", context, err)
	}

	return nil
}

func errorsIsDirectoryNotEmpty(err error) bool {
	return err != nil && (errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST))
}
