// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

var errPreviousMetadataNotFound = errors.New("previous metadata not found")

// LoadMetadata reads TaskOtter metadata from rel under workspace.
func LoadMetadata(workspace, rel string) (*Metadata, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("read metadata %q: %w", rel, err)
	}

	var meta Metadata

	err = yaml.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("parse metadata %q: %w", rel, err)
	}

	return &meta, nil
}

// LoadLock reads a TaskOtter lock file from rel under workspace.
func LoadLock(workspace, rel string) (*LockFile, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf("read lock file %q: %w", rel, err)
	}

	var lock LockFile

	err = yaml.Unmarshal(data, &lock)
	if err != nil {
		return nil, fmt.Errorf("parse lock file %q: %w", rel, err)
	}

	return &lock, nil
}

func loadPreviousState(workspace string, cfg *config.Config) (*LockFile, *Metadata, string, error) {
	oldMeta, err := loadCurrentMetadata(workspace, cfg)

	if errors.Is(err, errPreviousMetadataNotFound) {
		oldMeta = nil
	} else if err != nil {
		return nil, nil, "", err
	}

	oldLock, oldTarget, err := loadPreviousLock(workspace, cfg, oldMeta)
	if err != nil {
		return nil, nil, "", err
	}

	return oldLock, oldMeta, oldTarget, nil
}

func loadCurrentMetadata(workspace string, cfg *config.Config) (*Metadata, error) {
	meta, found, err := loadMetadataIfExists(workspace, cfg.MetadataPath())

	if err != nil || found {
		return meta, err
	}

	if cfg.MetadataPath() != config.LegacyMetadataPath {
		meta, found, err = loadMetadataIfExists(workspace, config.LegacyMetadataPath)

		if err != nil || found {
			return meta, err
		}
	}

	meta, err = discoverPreviousMetadata(workspace, cfg.MetadataPath())

	return meta, err
}

func loadMetadataIfExists(workspace, metadataPath string) (*Metadata, bool, error) {
	meta, err := LoadMetadata(workspace, metadataPath)

	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, err
	}

	return meta, true, nil
}

func loadPreviousLock(
	workspace string,
	cfg *config.Config,
	oldMeta *Metadata,
) (*LockFile, string, error) {
	oldLockPath := cfg.LockFilePath()
	oldTarget := ""

	if oldMeta != nil {
		if oldMeta.LockFile != "" {
			oldLockPath = oldMeta.LockFile
		}

		oldTarget = oldMeta.TargetFolder
	}

	oldLock, err := LoadLock(workspace, oldLockPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}

	if oldLock != nil && oldTarget == "" {
		oldTarget = oldLock.Configuration.TargetFolder
	}

	return oldLock, oldTarget, nil
}

func discoverPreviousMetadata(workspace, currentMetadataPath string) (*Metadata, error) {
	var candidates []string

	err := filepath.WalkDir(workspace, func(abs string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %q: %w", abs, walkErr)
		}

		rel, candidate, err := previousMetadataCandidate(workspace, currentMetadataPath, abs, entry)
		if err != nil {
			return err
		}

		if candidate {
			candidates = append(candidates, rel)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover previous metadata: %w", err)
	}

	sort.Strings(candidates)

	if len(candidates) == 0 {
		return nil, errPreviousMetadataNotFound
	}

	meta, err := LoadMetadata(workspace, candidates[0])
	if err != nil {
		return nil, fmt.Errorf("load previous metadata %q: %w", candidates[0], err)
	}

	return meta, nil
}

func previousMetadataCandidate(
	workspace, currentMetadataPath, abs string,
	entry os.DirEntry,
) (string, bool, error) {
	if entry.IsDir() {
		if entry.Name() == ".git" {
			return "", false, filepath.SkipDir
		}

		return "", false, nil
	}

	rel, err := filepath.Rel(workspace, abs)
	if err != nil {
		return "", false, fmt.Errorf("relative metadata path for %q: %w", abs, err)
	}

	rel = filepath.ToSlash(rel)

	if rel == currentMetadataPath || rel == config.LegacyMetadataPath {
		return "", false, nil
	}

	return rel, isTaskOtterMetadataPath(rel), nil
}

func isTaskOtterMetadataPath(rel string) bool {
	return filepath.Base(rel) == "metadata.yml" &&
		filepath.Base(filepath.Dir(rel)) == ".taskotter"
}
