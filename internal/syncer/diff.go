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
	"github.com/task-otter/Taskotter/internal/pathutil"
	"gopkg.in/yaml.v3"
)

func lockContentChanged(old *LockFile, newLock *LockFile) (bool, error) {
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
) ([]string, []string, []string, error) {
	current := make(map[string]ManagedFile, len(plan.ManagedFiles))
	for _, managed := range plan.ManagedFiles {
		current[managed.Path] = managed
	}

	var removed []string

	if plan.OldLock != nil {
		for _, managed := range plan.OldLock.ManagedFiles {
			if _, ok := current[managed.Path]; !ok {
				removed = append(removed, managed.Path)
			}
		}
	}

	added, updated, err := diffManagedFilePaths(current, workspace)
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

	lockPath := plan.Metadata.LockFile

	added, updated, err = diffLockFile(plan, workspace, lockPath, added, updated)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("diff lock file: %w", err)
	}

	added, updated, err = diffMetadataFile(workspace, metadataPath, plannedMeta, added, updated)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("diff metadata file: %w", err)
	}

	sort.Strings(added)
	sort.Strings(updated)
	sort.Strings(removed)

	return added, updated, removed, nil
}

func diffManagedFilePaths(
	current map[string]ManagedFile,
	workspace string,
) ([]string, []string, error) {
	var added, updated []string

	for path, managed := range current {
		data, readErr := pathutil.ReadRelativeFile(workspace, path)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				added = append(added, path)

				continue
			}

			return nil, nil, fmt.Errorf("read managed file %q: %w", path, readErr)
		}

		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != managed.SHA256 {
			updated = append(updated, path)
		}
	}

	return added, updated, nil
}

func diffRootTaskfile(
	oldRoot, newRoot []byte,
	rootPath string,
	added, updated []string,
) ([]string, []string) {
	if bytes.Equal(oldRoot, newRoot) {
		return added, updated
	}

	if len(oldRoot) == 0 {
		return append(added, rootPath), updated
	}

	return added, append(updated, rootPath)
}

func diffLockFile(
	plan *Plan,
	workspace, lockPath string,
	added, updated []string,
) ([]string, []string, error) {
	oldLockBytes, readErr := pathutil.ReadRelativeFile(workspace, lockPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("read lock file %q: %w", lockPath, readErr)
	}

	changed, err := lockContentChanged(plan.OldLock, &plan.Lock)
	if err != nil {
		return nil, nil, err
	}

	if !changed {
		return added, updated, nil
	}

	if len(oldLockBytes) == 0 {
		return append(added, lockPath), updated, nil
	}

	return added, append(updated, lockPath), nil
}

func diffMetadataFile(
	workspace, metadataPath string,
	plannedMeta []byte,
	added, updated []string,
) ([]string, []string, error) {
	oldMetaBytes, readErr := pathutil.ReadRelativeFile(workspace, metadataPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("read metadata %q: %w", metadataPath, readErr)
	}

	if bytes.Equal(oldMetaBytes, plannedMeta) {
		return added, updated, nil
	}

	if len(oldMetaBytes) == 0 {
		return append(added, metadataPath), updated, nil
	}

	return added, append(updated, metadataPath), nil
}

func buildStagePaths(plan *Plan, workspace, metadataPath string, syncRoot bool) []string {
	paths := make(map[string]struct{})
	for _, managed := range plan.ManagedFiles {
		paths[managed.Path] = struct{}{}
	}

	for _, rm := range plan.Removed {
		paths[rm] = struct{}{}
	}

	if syncRoot {
		paths[plan.RootTaskfilePath] = struct{}{}
	}

	paths[plan.Metadata.LockFile] = struct{}{}

	addMetadataStagePaths(paths, workspace, metadataPath)
	addOldTargetStagePaths(paths, plan, workspace)

	return sortedStagePaths(paths)
}

func addMetadataStagePaths(paths map[string]struct{}, workspace, metadataPath string) {
	paths[metadataPath] = struct{}{}
	if metadataPath != config.LegacyMetadataPath &&
		relativePathExists(workspace, config.LegacyMetadataPath) {
		paths[config.LegacyMetadataPath] = struct{}{}
	}
}

func addOldTargetStagePaths(paths map[string]struct{}, plan *Plan, workspace string) {
	if plan.OldLock != nil && plan.OldTargetFolder != "" &&
		plan.OldTargetFolder != plan.Metadata.TargetFolder {
		for _, managed := range plan.OldLock.ManagedFiles {
			if pathutil.HasFolderPrefix(managed.Path, plan.OldTargetFolder) {
				paths[managed.Path] = struct{}{}
			}
		}

		oldLockPath := pathutil.JoinRelative(plan.OldTargetFolder, ".taskotter-lock.yml")
		paths[oldLockPath] = struct{}{}

		oldMetadataPath := pathutil.JoinRelative(plan.OldTargetFolder, ".taskotter/metadata.yml")
		if relativePathExists(workspace, oldMetadataPath) {
			paths[oldMetadataPath] = struct{}{}
		}
	}
}

func sortedStagePaths(paths map[string]struct{}) []string {
	out := make([]string, 0, len(paths))
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
