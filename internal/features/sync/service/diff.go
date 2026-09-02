// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

const (
	errReadLockFile  = "read lock file %q: %w"
	errReadMetadata  = "read metadata %q: %w"
	errWalk          = "walk %q: %w"
	taskfilesDirName = "taskfiles"
)

func lockContentChanged(old, newLock *syncLock) (changed bool, err error) {
	oldNorm, err := marshalLockForCompare(old)
	if err != nil {
		return false, fmt.Errorf("normalize old lock file: %w", err)
	}

	newNorm, err := marshalLockForCompare(newLock)
	if err != nil {
		return false, fmt.Errorf("normalize new lock file: %w", err)
	}

	return !bytes.Equal(oldNorm, newNorm), nil
}

func marshalLockForCompare(lock *syncLock) ([]byte, error) {
	if lock == nil {
		return nil, nil
	}

	cloned := *lock

	cloned.Source.ResolvedCommit = consts.Empty

	data, err := marshalYAML(lockmodel.EncodeLockFile(&cloned))
	if err != nil {
		return nil, fmt.Errorf("marshal lock file for compare: %w", err)
	}

	return data, nil
}

func diffFiles(input *diffInput) (diffLists, error) {
	current, removed := currentAndRemoved(input.plan)

	lists, err := diffManagedFilePaths(current, input.workspace)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff managed files: %w", err)
	}

	lists.removed = removed
	lists = *diffRootIfEnabled(input, &lists)

	lists, err = diffLockAndMetadata(input, &lists)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff lock and metadata: %w", err)
	}

	return *sortDiffLists(&lists), nil
}

func diffRootIfEnabled(input *diffInput, lists *diffLists) *diffLists {
	if input.syncRoot != syncRootEnabled {
		return lists
	}

	return diffRootTaskfile(&diffRootInput{
		oldRoot:  input.oldRoot,
		newRoot:  input.plan.RootTaskfile,
		rootPath: input.plan.RootTaskfilePath,
		lists:    *lists,
	})
}

func currentAndRemoved(plan *domain.Plan) (current map[string]managedFile, removed []string) {
	current = buildCurrentManagedFiles(plan)
	removed = diffRemovedFiles(plan.OldLock, current)

	return current, removed
}

func buildCurrentManagedFiles(plan *domain.Plan) map[string]managedFile {
	current := make(map[string]managedFile, len(plan.ManagedFiles))

	for i := range plan.ManagedFiles {
		managed := &plan.ManagedFiles[i]

		current[managed.Path] = *managed
	}

	return current
}

func diffRemovedFiles(oldLock *syncLock, current map[string]managedFile) []string {
	var removed []string

	if oldLock == nil {
		return removed
	}

	for i := range oldLock.ManagedFiles {
		managed := &oldLock.ManagedFiles[i]

		existing, ok := current[managed.Path]
		iox.Discard(existing)

		if !ok {
			removed = append(removed, managed.Path)
		}
	}

	return removed
}

func diffLockAndMetadata(input *diffInput, lists *diffLists) (diffLists, error) {
	listsResult, err := diffLockFileSection(input, lists)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff lock file section: %w", err)
	}

	listsResult, err = diffMetadataFileSection(input, &listsResult)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff metadata file section: %w", err)
	}

	return listsResult, nil
}

func diffLockFileSection(input *diffInput, lists *diffLists) (diffLists, error) {
	listsResult, err := diffLockFile(&diffLockArgs{
		plan:      input.plan,
		workspace: input.workspace,
		lockPath:  input.plan.Metadata.LockFile,
		lists:     *lists,
	})
	if err != nil {
		return diffLists{}, fmt.Errorf("diff lock file: %w", err)
	}

	return listsResult, nil
}

func diffMetadataFileSection(input *diffInput, lists *diffLists) (diffLists, error) {
	listsResult, err := diffMetadataFile(&diffMetadataArgs{
		workspace:    input.workspace,
		metadataPath: input.metadataPath,
		plannedMeta:  input.plannedMeta,
		lists:        *lists,
	})
	if err != nil {
		return diffLists{}, fmt.Errorf("diff metadata file: %w", err)
	}

	return listsResult, nil
}

func sortDiffLists(lists *diffLists) *diffLists {
	slices.Sort(lists.added)
	slices.Sort(lists.updated)
	slices.Sort(lists.removed)

	return lists
}

func diffManagedFilePaths(current map[string]managedFile, workspace string) (diffLists, error) {
	var lists diffLists

	for path := range current {
		managed := current[path]

		change, changeErr := fileChanged(workspace, path, &managed)
		if changeErr != nil {
			return diffLists{}, fmt.Errorf("read managed file %q: %w", path, changeErr)
		}

		lists = *applyFileChange(&lists, path, change)
	}

	return lists, nil
}

func applyFileChange(lists *diffLists, path string, change fileChangeKind) *diffLists {
	switch change { //nolint:revive // exhaustive cases include intentional no-op branches
	case fileAdded:
		lists.added = append(lists.added, path)
	case fileUpdated:
		lists.updated = append(lists.updated, path)
	case fileUnchanged:
	default:
	}

	return lists
}

func fileChanged(workspace, path string, managed *managedFile) (fileChangeKind, error) {
	data, readErr := pathutil.ReadRelativeFile(workspace, path)
	if readErr == nil {
		return fileChangeFromData(data, managed), nil
	}

	if errors.Is(readErr, os.ErrNotExist) {
		return fileAdded, nil
	}

	return fileUnchanged, fmt.Errorf(errFmtReadQuoted, path, readErr)
}

func fileChangeFromData(data []byte, managed *managedFile) fileChangeKind {
	sum := sha256.Sum256(data)

	if hex.EncodeToString(sum[:]) != managed.SHA256 {
		return fileUpdated
	}

	return fileUnchanged
}

func diffRootTaskfile(input *diffRootInput) *diffLists {
	if bytes.Equal(input.oldRoot, input.newRoot) {
		return &input.lists
	}

	if len(input.oldRoot) == consts.IndexZero {
		input.lists.added = append(input.lists.added, input.rootPath)

		return &input.lists
	}

	input.lists.updated = append(input.lists.updated, input.rootPath)

	return &input.lists
}

func classifyAddedOrUpdated(prior priorContent, path string, lists *diffLists) *diffLists {
	if prior == priorContentEmpty {
		lists.added = append(lists.added, path)

		return lists
	}

	lists.updated = append(lists.updated, path)

	return lists
}

func priorContentFromBytes(data []byte) priorContent {
	if len(data) == consts.IndexZero {
		return priorContentEmpty
	}

	return priorContentExists
}

func diffLockFile(args *diffLockArgs) (diffLists, error) {
	oldLockBytes, readErr := pathutil.ReadRelativeFile(args.workspace, args.lockPath)

	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return diffLists{}, fmt.Errorf(errReadLockFile, args.lockPath, readErr)
	}

	changed, err := lockContentChanged(args.plan.OldLock, &args.plan.Lock)
	if err != nil {
		return diffLists{}, fmt.Errorf("lock content changed: %w", err)
	}

	if !changed {
		return args.lists, nil
	}

	return *classifyAddedOrUpdated(
		priorContentFromBytes(oldLockBytes),
		args.lockPath,
		&args.lists,
	), nil
}

func diffMetadataFile(args *diffMetadataArgs) (diffLists, error) {
	oldMetaBytes, readErr := pathutil.ReadRelativeFile(args.workspace, args.metadataPath)

	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return diffLists{}, fmt.Errorf(errReadMetadata, args.metadataPath, readErr)
	}

	if bytes.Equal(oldMetaBytes, args.plannedMeta) {
		return args.lists, nil
	}

	return *classifyAddedOrUpdated(
		priorContentFromBytes(oldMetaBytes),
		args.metadataPath,
		&args.lists,
	), nil
}

func buildStagePaths(input *stagePathsInput) []string {
	paths := baseStagePaths(input.plan, input.syncRoot)

	addMetadataStagePaths(paths, input.workspace, input.metadataPath)
	addOldTargetStagePaths(paths, input.plan, input.workspace)

	return sortedStagePaths(paths)
}

func baseStagePaths(plan *domain.Plan, syncRoot syncRootPolicy) map[string]struct{} {
	paths := make(map[string]struct{})

	for i := range plan.ManagedFiles {
		paths[plan.ManagedFiles[i].Path] = struct{}{}
	}

	for i := range plan.Removed {
		paths[plan.Removed[i]] = struct{}{}
	}

	if syncRoot == syncRootEnabled {
		paths[plan.RootTaskfilePath] = struct{}{}
	}

	paths[plan.Metadata.LockFile] = struct{}{}

	return paths
}

func addMetadataStagePaths(paths map[string]struct{}, workspace, metadataPath string) {
	paths[metadataPath] = struct{}{}

	if shouldStageLegacyMetadata(workspace, metadataPath) {
		paths[config.LegacyMetadataPath] = struct{}{}
	}
}

func shouldStageLegacyMetadata(workspace, metadataPath string) bool {
	return metadataPath != config.LegacyMetadataPath &&
		relativePathExists(workspace, config.LegacyMetadataPath)
}

func addOldTargetStagePaths(paths map[string]struct{}, plan *domain.Plan, workspace string) {
	if oldTargetUnchanged(plan) {
		return
	}

	addOldManagedFilePaths(paths, plan.OldLock, plan.OldTargetFolder)

	oldLockPath := pathutil.JoinRelative(plan.OldTargetFolder, lockFileName)

	paths[oldLockPath] = struct{}{}

	oldMetadataPath := pathutil.JoinRelative(plan.OldTargetFolder, legacyMetadataRelPath)

	if relativePathExists(workspace, oldMetadataPath) {
		paths[oldMetadataPath] = struct{}{}
	}
}

func addOldManagedFilePaths(paths map[string]struct{}, oldLock *syncLock, oldTargetFolder string) {
	for i := range oldLock.ManagedFiles {
		managed := &oldLock.ManagedFiles[i]

		if pathutil.HasFolderPrefix(managed.Path, oldTargetFolder) {
			paths[managed.Path] = struct{}{}
		}
	}
}

func sortedStagePaths(paths map[string]struct{}) []string {
	out := make([]string, consts.IndexZero, len(paths))

	for relPath := range paths {
		out = append(out, relPath)
	}

	slices.Sort(out)

	return out
}

func relativePathExists(workspace, rel string) bool {
	info, err := statPath(pathutil.WorkspacePath(workspace, rel))
	iox.Discard(info)

	return err == nil
}
