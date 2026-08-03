// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

const (
	errReadLockFile  = "read lock file %q: %w"
	errReadMetadata  = "read metadata %q: %w"
	errWalk          = "walk %q: %w"
	taskfilesDirName = "taskfiles"
)

func lockContentChanged(old, newLock *LockFile) (changed bool, err error) {
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

func marshalLockForCompare(lock *LockFile) ([]byte, error) {
	if lock == nil {
		return nil, nil
	}

	cloned := *lock

	cloned.Source.ResolvedCommit = ""

	data, err := yaml.Marshal(cloned)
	if err != nil {
		return nil, fmt.Errorf("marshal lock file for compare: %w", err)
	}

	return data, nil
}

func diffFiles(
	plan *Plan,
	workspace string,
	oldRoot []byte,
	syncRoot bool,
	metadataPath string,
	plannedMeta []byte,
) (added, updated, removed []string, err error) {
	var current map[string]ManagedFile

	current, removed = currentAndRemoved(plan)

	added, updated, err = diffManagedFilePaths(current, workspace)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("diff managed files: %w", err)
	}

	if syncRoot {
		added, updated = diffRootTaskfile(
			oldRoot,
			plan.RootTaskfile,
			plan.RootTaskfilePath,
			added,
			updated,
		)
	}

	added, updated, err = diffLockAndMetadata(
		plan,
		workspace,
		metadataPath,
		plannedMeta,
		added,
		updated,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("diff lock and metadata: %w", err)
	}

	added, updated, removed, err = finalizeDiff(added, updated, removed)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("finalize diff: %w", err)
	}

	return added, updated, removed, nil
}

func currentAndRemoved(plan *Plan) (current map[string]ManagedFile, removed []string) {
	current = buildCurrentManagedFiles(plan)
	removed = diffRemovedFiles(plan.OldLock, current)

	return current, removed
}

func buildCurrentManagedFiles(plan *Plan) map[string]ManagedFile {
	current := make(map[string]ManagedFile, len(plan.ManagedFiles))

	for i := range plan.ManagedFiles {
		managed := &plan.ManagedFiles[i]

		current[managed.Path] = *managed
	}

	return current
}

func diffRemovedFiles(oldLock *LockFile, current map[string]ManagedFile) []string {
	var removed []string

	if oldLock == nil {
		return removed
	}

	for i := range oldLock.ManagedFiles {
		managed := &oldLock.ManagedFiles[i]

		if _, ok := current[managed.Path]; !ok {
			removed = append(removed, managed.Path)
		}
	}

	return removed
}

func diffLockAndMetadata(
	plan *Plan,
	workspace, metadataPath string,
	plannedMeta []byte,
	added, updated []string,
) (newAdded, newUpdated []string, err error) {
	lockPath := plan.Metadata.LockFile

	added, updated, err = diffLockFile(plan, workspace, lockPath, added, updated)
	if err != nil {
		return nil, nil, fmt.Errorf("diff lock file: %w", err)
	}

	added, updated, err = diffMetadataFile(workspace, metadataPath, plannedMeta, added, updated)
	if err != nil {
		return nil, nil, fmt.Errorf("diff metadata file: %w", err)
	}

	return added, updated, nil
}

func finalizeDiff(added, updated, removed []string) (a, u, r []string, err error) {
	sort.Strings(added)
	sort.Strings(updated)
	sort.Strings(removed)

	return added, updated, removed, nil
}

func diffManagedFilePaths(
	current map[string]ManagedFile,
	workspace string,
) (added, updated []string, err error) {
	for path := range current {
		managed := current[path]

		isAdded, isUpdated, changeErr := fileChanged(workspace, path, &managed)
		if changeErr != nil {
			return nil, nil, fmt.Errorf("read managed file %q: %w", path, changeErr)
		}

		added, updated = appendFileChange(added, updated, path, isAdded, isUpdated)
	}

	return added, updated, nil
}

func appendFileChange(
	added, updated []string,
	path string,
	isAdded, isUpdated bool,
) (newAdded, newUpdated []string) {
	if isAdded {
		return append(added, path), updated
	}

	if isUpdated {
		return added, append(updated, path)
	}

	return added, updated
}

func fileChanged(workspace, path string, managed *ManagedFile) (added, updated bool, err error) {
	data, readErr := pathutil.ReadRelativeFile(workspace, path)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return true, false, nil
		}

		return false, false, fmt.Errorf("read %q: %w", path, readErr)
	}

	sum := sha256.Sum256(data)

	return false, hex.EncodeToString(sum[:]) != managed.SHA256, nil
}

func diffRootTaskfile(
	oldRoot, newRoot []byte,
	rootPath string,
	added, updated []string,
) (newA, newU []string) {
	if bytes.Equal(oldRoot, newRoot) {
		return added, updated
	}

	if len(oldRoot) == consts.IndexZero {
		return append(added, rootPath), updated
	}

	return added, append(updated, rootPath)
}

// classifyAddedOrUpdated appends path to added (when the previous content was empty,
// i.e. the file didn't exist before) or to updated (otherwise).
func classifyAddedOrUpdated(
	oldBytesEmpty bool,
	path string,
	added, updated []string,
) (newAdded, newUpdated []string) {
	if oldBytesEmpty {
		return append(added, path), updated
	}

	return added, append(updated, path)
}

func diffLockFile(
	plan *Plan,
	workspace, lockPath string,
	added, updated []string,
) (newAdded, newUpdated []string, err error) {
	oldLockBytes, readErr := pathutil.ReadRelativeFile(workspace, lockPath)

	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, nil, fmt.Errorf(errReadLockFile, lockPath, readErr)
	}

	changed, err := lockContentChanged(plan.OldLock, &plan.Lock)
	if err != nil {
		return nil, nil, fmt.Errorf("lock content changed: %w", err)
	}

	if !changed {
		return added, updated, nil
	}

	newAdded, newUpdated = classifyAddedOrUpdated(
		len(oldLockBytes) == consts.IndexZero,
		lockPath,
		added,
		updated,
	)

	return newAdded, newUpdated, nil
}

func diffMetadataFile(
	workspace, metadataPath string,
	plannedMeta []byte,
	added, updated []string,
) (newAdded, newUpdated []string, err error) {
	oldMetaBytes, readErr := pathutil.ReadRelativeFile(workspace, metadataPath)

	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, nil, fmt.Errorf(errReadMetadata, metadataPath, readErr)
	}

	if bytes.Equal(oldMetaBytes, plannedMeta) {
		return added, updated, nil
	}

	newAdded, newUpdated = classifyAddedOrUpdated(
		len(oldMetaBytes) == consts.IndexZero,
		metadataPath,
		added,
		updated,
	)

	return newAdded, newUpdated, nil
}

func buildStagePaths(plan *Plan, workspace, metadataPath string, syncRoot bool) []string {
	paths := baseStagePaths(plan, syncRoot)

	addMetadataStagePaths(paths, workspace, metadataPath)
	addOldTargetStagePaths(paths, plan, workspace)

	return sortedStagePaths(paths)
}

func baseStagePaths(plan *Plan, syncRoot bool) map[string]struct{} {
	paths := make(map[string]struct{})

	for i := range plan.ManagedFiles {
		paths[plan.ManagedFiles[i].Path] = struct{}{}
	}

	for i := range plan.Removed {
		paths[plan.Removed[i]] = struct{}{}
	}

	if syncRoot {
		paths[plan.RootTaskfilePath] = struct{}{}
	}

	paths[plan.Metadata.LockFile] = struct{}{}

	return paths
}

func addMetadataStagePaths(paths map[string]struct{}, workspace, metadataPath string) {
	paths[metadataPath] = struct{}{}

	if metadataPath != config.LegacyMetadataPath &&
		relativePathExists(workspace, config.LegacyMetadataPath) {
		paths[config.LegacyMetadataPath] = struct{}{}
	}
}

func addOldTargetStagePaths(paths map[string]struct{}, plan *Plan, workspace string) {
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

func addOldManagedFilePaths(paths map[string]struct{}, oldLock *LockFile, oldTargetFolder string) {
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

	sort.Strings(out)

	return out
}

func relativePathExists(workspace, rel string) bool {
	_, err := os.Stat(pathutil.WorkspacePath(workspace, rel))

	return err == nil
}
