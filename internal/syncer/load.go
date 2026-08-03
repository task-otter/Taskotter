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
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

var errPreviousMetadataNotFound = errors.New("previous metadata not found")

// LoadMetadata reads TaskOtter metadata from rel under workspace.
func LoadMetadata(workspace, rel string) (*Metadata, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf(errReadMetadata, rel, err)
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
		return nil, fmt.Errorf(errReadLockFile, rel, err)
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
		return nil, nil, consts.Empty, fmt.Errorf("load current metadata: %w", err)
	}

	oldLock, oldTarget, err := loadPreviousLock(workspace, cfg, oldMeta)
	if err != nil {
		return nil, nil, consts.Empty, fmt.Errorf("load previous lock: %w", err)
	}

	return oldLock, oldMeta, oldTarget, nil
}

func loadCurrentMetadata(workspace string, cfg *config.Config) (*Metadata, error) {
	meta, found, err := loadMetadataIfExists(workspace, cfg.MetadataPath())
	if err != nil {
		return meta, fmt.Errorf("load metadata: %w", err)
	}

	if found {
		return meta, nil
	}

	meta, found, err = tryLegacyMetadata(workspace, cfg.MetadataPath())
	if err != nil {
		return meta, fmt.Errorf("try legacy metadata: %w", err)
	}

	if found {
		return meta, nil
	}

	discovered, err := discoverPreviousMetadata(workspace, cfg.MetadataPath())
	if err != nil {
		return nil, fmt.Errorf("discover previous metadata: %w", err)
	}

	return discovered, nil
}

func tryLegacyMetadata(workspace, currentMetadataPath string) (*Metadata, bool, error) {
	if currentMetadataPath == config.LegacyMetadataPath {
		return nil, false, nil
	}

	meta, found, err := loadMetadataIfExists(workspace, config.LegacyMetadataPath)
	if err != nil {
		return nil, false, fmt.Errorf("load legacy metadata: %w", err)
	}

	return meta, found, nil
}

func loadMetadataIfExists(workspace, metadataPath string) (*Metadata, bool, error) {
	meta, err := LoadMetadata(workspace, metadataPath)

	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}

	if err != nil {
		return nil, false, fmt.Errorf("load metadata %q: %w", metadataPath, err)
	}

	return meta, true, nil
}

func loadPreviousLock(
	workspace string,
	cfg *config.Config,
	oldMeta *Metadata,
) (*LockFile, string, error) {
	oldLockPath, oldTarget := resolveOldLockPathAndTarget(cfg, oldMeta)

	oldLock, err := LoadLock(workspace, oldLockPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, consts.Empty, fmt.Errorf("load lock %q: %w", oldLockPath, err)
	}

	if oldLock != nil && oldTarget == consts.Empty {
		oldTarget = oldLock.Configuration.TargetFolder
	}

	return oldLock, oldTarget, nil
}

func resolveOldLockPathAndTarget(cfg *config.Config, oldMeta *Metadata) (string, string) {
	oldLockPath := cfg.LockFilePath()

	if oldMeta == nil {
		return oldLockPath, consts.Empty
	}

	if oldMeta.LockFile != consts.Empty {
		oldLockPath = oldMeta.LockFile
	}

	return oldLockPath, oldMeta.TargetFolder
}

func discoverPreviousMetadata(workspace, currentMetadataPath string) (*Metadata, error) {
	candidates, err := collectMetadataCandidates(workspace, currentMetadataPath)
	if err != nil {
		return nil, fmt.Errorf("collect metadata candidates: %w", err)
	}

	if len(candidates) == consts.IndexZero {
		return nil, errPreviousMetadataNotFound
	}

	meta, err := loadFirstCandidate(workspace, candidates)
	if err != nil {
		return nil, fmt.Errorf("load first candidate: %w", err)
	}

	return meta, nil
}

func collectMetadataCandidates(workspace, currentMetadataPath string) ([]string, error) {
	var candidates []string

	walker := metadataCandidateWalker(workspace, currentMetadataPath, &candidates)

	err := filepath.WalkDir(workspace, walker)
	if err != nil {
		return nil, fmt.Errorf("discover previous metadata: %w", err)
	}

	sort.Strings(candidates)

	return candidates, nil
}

func metadataCandidateWalker(
	workspace, currentMetadataPath string,
	candidates *[]string,
) func(string, os.DirEntry, error) error {
	return func(abs string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf(errWalk, abs, walkErr)
		}

		rel, candidate, err := previousMetadataCandidate(workspace, currentMetadataPath, abs, entry)
		if err != nil {
			if errors.Is(err, filepath.SkipDir) {
				return filepath.SkipDir
			}

			return fmt.Errorf("previous metadata candidate: %w", err)
		}

		appendCandidate(candidates, rel, candidate)

		return nil
	}
}

func appendCandidate(candidates *[]string, rel string, isCandidate bool) {
	if isCandidate {
		*candidates = append(*candidates, rel)
	}
}

func loadFirstCandidate(workspace string, candidates []string) (*Metadata, error) {
	meta, err := LoadMetadata(workspace, candidates[consts.IndexZero])
	if err != nil {
		return nil, fmt.Errorf("load previous metadata %q: %w", candidates[consts.IndexZero], err)
	}

	return meta, nil
}

func previousMetadataCandidate(
	workspace, currentMetadataPath, abs string,
	entry os.DirEntry,
) (string, bool, error) {
	if entry.IsDir() {
		//nolint:wrapcheck // must return filepath.SkipDir sentinel unwrapped
		return handleDirEntry(entry)
	}

	rel, err := filepath.Rel(workspace, abs)
	if err != nil {
		return consts.Empty, false, fmt.Errorf("relative metadata path for %q: %w", abs, err)
	}

	rel = filepath.ToSlash(rel)

	if isCurrentOrLegacyMetadataPath(rel, currentMetadataPath) {
		return consts.Empty, false, nil
	}

	return rel, isTaskOtterMetadataPath(rel), nil
}

func handleDirEntry(entry os.DirEntry) (string, bool, error) {
	if entry.Name() == ".git" {
		return consts.Empty, false, filepath.SkipDir
	}

	return consts.Empty, false, nil
}

func isCurrentOrLegacyMetadataPath(rel, currentMetadataPath string) bool {
	return rel == currentMetadataPath || rel == config.LegacyMetadataPath
}

func isTaskOtterMetadataPath(rel string) bool {
	return filepath.Base(rel) == storeMetadataFileName &&
		filepath.Base(filepath.Dir(rel)) == legacyMetadataDirName
}
