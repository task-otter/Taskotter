package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/pathutil"
	"github.com/task-otter/Taskotter/internal/yamlfmt"
	"gopkg.in/yaml.v3"
)

type stagedFile struct {
	finalRel string
	entry    FileEntry
}

func buildStagedFiles(plan *Plan, syncInput SyncInput) ([]stagedFile, error) {
	var staged []stagedFile

	for _, mod := range sortedModuleRecords(plan.Requested, plan.Dependencies) {
		contents := plan.ModuleContents[mod.SourceModule]

		rels := make([]string, 0, len(contents))
		for rel := range contents {
			rels = append(rels, rel)
		}

		sort.Strings(rels)

		destDirRel := pathutil.JoinRelative(syncInput.Config.TargetFolder, mod.DestinationModule)
		for _, rel := range rels {
			finalRel := pathutil.JoinRelative(destDirRel, rel)
			staged = append(staged, stagedFile{finalRel: finalRel, entry: contents[rel]})
		}
	}

	if syncInput.Config.SyncRoot {
		staged = append(staged, stagedFile{
			finalRel: plan.RootTaskfilePath,
			entry:    FileEntry{Data: plan.RootTaskfile, Mode: fileModeRegular},
		})
	}

	lockBytes, err := yamlfmt.Marshal(plan.Lock)
	if err != nil {
		return nil, fmt.Errorf("marshal lock file: %w", err)
	}

	staged = append(staged, stagedFile{
		finalRel: plan.Metadata.LockFile,
		entry:    FileEntry{Data: lockBytes, Mode: fileModeRegular},
	})

	metaBytes, err := yamlfmt.Marshal(plan.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	staged = append(staged, stagedFile{
		finalRel: syncInput.Config.MetadataPath(),
		entry:    FileEntry{Data: metaBytes, Mode: fileModeRegular},
	})

	return staged, nil
}

// ApplyPlan writes planned files atomically and removes obsolete managed paths.
func ApplyPlan(plan *Plan, syncInput SyncInput) error {
	workspace := syncInput.Config.Workspace

	staged, err := buildStagedFiles(plan, syncInput)
	if err != nil {
		return err
	}

	copyFile := plan.copyFileTo
	if copyFile == nil {
		copyFile = copyFileTo
	}

	stagingRoot, err := stagePlanFiles(staged, workspace, syncInput.Config.TargetFolder, copyFile)
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stagingRoot) }()

	err = validateGeneratedYAML(staged, plan.RootTaskfilePath)
	if err != nil {
		return err
	}

	err = writeStagedFiles(staged, workspace, copyFile)
	if err != nil {
		return err
	}

	err = removeObsolete(plan, workspace)
	if err != nil {
		return err
	}

	return cleanupLegacyMetadata(workspace, syncInput.Config.MetadataPath())
}

func stagePlanFiles(
	staged []stagedFile,
	workspace, targetFolder string,
	copyFile func(string, FileEntry) error,
) (string, error) {
	stagingParent := pathutil.WorkspacePath(
		workspace,
		pathutil.JoinRelative(targetFolder, ".taskotter/staging"),
	)

	err := os.MkdirAll(stagingParent, dirModePerm)
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}

	stagingRoot, err := os.MkdirTemp(stagingParent, "apply-*")
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}

	for _, stagedEntry := range staged {
		stagePath := filepath.Join(stagingRoot, filepath.FromSlash(stagedEntry.finalRel))

		err = copyFile(stagePath, stagedEntry.entry)
		if err != nil {
			_ = os.RemoveAll(stagingRoot)

			return "", fmt.Errorf("stage %q: %w", stagedEntry.finalRel, err)
		}
	}

	return stagingRoot, nil
}

func writeStagedFiles(
	staged []stagedFile,
	workspace string,
	copyFile func(string, FileEntry) error,
) error {
	for _, stagedEntry := range staged {
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
	for _, stagedEntry := range staged {
		switch {
		case stagedEntry.finalRel == rootPath:
			var node yaml.Node

			err := yaml.Unmarshal(stagedEntry.entry.Data, &node)
			if err != nil {
				return fmt.Errorf("validate root Taskfile.yml: %w", err)
			}
		case filepath.Base(stagedEntry.finalRel) == ".taskotter-lock.yml":
			var lock LockFile

			err := yaml.Unmarshal(stagedEntry.entry.Data, &lock)
			if err != nil {
				return fmt.Errorf("validate lock file: %w", err)
			}
		case filepath.Base(stagedEntry.finalRel) == "metadata.yml":
			var meta Metadata

			err := yaml.Unmarshal(stagedEntry.entry.Data, &meta)
			if err != nil {
				return fmt.Errorf("validate metadata: %w", err)
			}
		}
	}

	return nil
}

func removeObsolete(plan *Plan, workspace string) error {
	currentPaths := make(map[string]struct{}, len(plan.ManagedFiles))
	for _, managed := range plan.ManagedFiles {
		currentPaths[managed.Path] = struct{}{}
	}

	if plan.OldLock == nil {
		return nil
	}

	for _, old := range plan.OldLock.ManagedFiles {
		if _, ok := currentPaths[old.Path]; ok {
			continue
		}

		abs := pathutil.WorkspacePath(workspace, old.Path)

		err := os.Remove(abs)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove obsolete file %q: %w", old.Path, err)
		}
	}

	return cleanupOldTarget(plan, workspace)
}

func cleanupOldTarget(plan *Plan, workspace string) error {
	if plan.OldLock == nil || plan.OldTargetFolder == "" ||
		plan.OldTargetFolder == plan.Metadata.TargetFolder {
		return nil
	}

	err := removeOldTargetFiles(plan, workspace)
	if err != nil {
		return err
	}

	err = removeIfExists(
		workspace,
		pathutil.JoinRelative(plan.OldTargetFolder, ".taskotter-lock.yml"),
	)
	if err != nil {
		return fmt.Errorf("remove old lock file: %w", err)
	}

	return removeOldTargetMetadata(plan, workspace)
}

func removeOldTargetFiles(plan *Plan, workspace string) error {
	for _, old := range plan.OldLock.ManagedFiles {
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
	oldMetadataRel := pathutil.JoinRelative(plan.OldTargetFolder, ".taskotter/metadata.yml")

	err := removeIfExists(workspace, oldMetadataRel)
	if err != nil {
		return fmt.Errorf("remove old metadata file: %w", err)
	}

	oldMetadata := pathutil.WorkspacePath(workspace, oldMetadataRel)

	err = os.Remove(filepath.Dir(oldMetadata))
	if err != nil && !os.IsNotExist(err) && !errorsIsDirectoryNotEmpty(err) {
		return fmt.Errorf("remove old metadata directory: %w", err)
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

	err = os.Remove(pathutil.WorkspacePath(workspace, ".taskotter"))
	if err != nil && !os.IsNotExist(err) && !errorsIsDirectoryNotEmpty(err) {
		return fmt.Errorf("remove legacy metadata directory: %w", err)
	}

	return nil
}

func errorsIsDirectoryNotEmpty(err error) bool {
	return err != nil && (errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST))
}
