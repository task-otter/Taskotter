// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

var (
	errPreviousMetadataNotFound = errors.New("previous metadata not found")
	errMetadataNotFound         = errors.New("metadata not found")
)

// LoadMetadata reads TaskOtter metadata from rel under workspace.
//

// LoadMetadata reads TaskOtter metadata from workspace-relative path rel.
func LoadMetadata(workspace, rel string) (*domain.Metadata, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf(errReadMetadata, rel, err)
	}

	var meta domain.Metadata

	err = yaml.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("parse metadata %q: %w", rel, err)
	}

	return &meta, nil
}

// LoadLock reads a TaskOtter lock file from rel under workspace.
//

// LoadLock reads the TaskOtter lock file from workspace-relative path rel.
func LoadLock(workspace, rel string) (*syncLock, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf(errReadLockFile, rel, err)
	}

	var lock syncLock

	err = yaml.Unmarshal(data, &lock)
	if err != nil {
		return nil, fmt.Errorf("parse lock file %q: %w", rel, err)
	}

	return &lock, nil
}

func loadPreviousState(workspace string, cfg *config.Config) (previousState, error) {
	oldMeta, err := loadCurrentMetadata(workspace, cfg)

	if err != nil && !errors.Is(err, errPreviousMetadataNotFound) {
		return previousState{}, fmt.Errorf("load current metadata: %w", err)
	}

	oldLock, oldTarget, err := loadPreviousLock(workspace, cfg, oldMeta)
	if err != nil {
		return previousState{}, fmt.Errorf("load previous lock: %w", err)
	}

	return previousState{lock: oldLock, target: oldTarget}, nil
}

func loadCurrentMetadata(workspace string, cfg *config.Config) (*domain.Metadata, error) {
	meta, found, err := loadMetadataIfExists(workspace, cfg.MetadataPath())

	if err != nil && !errors.Is(err, errMetadataNotFound) {
		return nil, fmt.Errorf("load metadata: %w", err)
	}

	if found {
		return meta, nil
	}

	meta, err = loadMetadataFallbacks(workspace, cfg.MetadataPath())
	if err != nil {
		return nil, fmt.Errorf("load metadata fallbacks: %w", err)
	}

	return meta, nil
}

func loadMetadataFallbacks(workspace, currentMetadataPath string) (*domain.Metadata, error) {
	meta, found, err := tryLegacyMetadata(workspace, currentMetadataPath)

	if err != nil && !errors.Is(err, errMetadataNotFound) {
		return nil, fmt.Errorf("try legacy metadata: %w", err)
	}

	if found {
		return meta, nil
	}

	discovered, err := discoverPreviousMetadata(workspace, currentMetadataPath)
	if err != nil {
		return nil, fmt.Errorf(errDiscoverPreviousMetadata, err)
	}

	return discovered, nil
}

func tryLegacyMetadata(workspace, currentMetadataPath string) (*domain.Metadata, bool, error) {
	if currentMetadataPath == config.LegacyMetadataPath {
		return nil, false, errMetadataNotFound
	}

	meta, found, err := loadMetadataIfExists(workspace, config.LegacyMetadataPath)

	if err != nil && !errors.Is(err, errMetadataNotFound) {
		return nil, false, fmt.Errorf("load legacy metadata: %w", err)
	}

	if !found {
		return nil, false, errMetadataNotFound
	}

	return meta, true, nil
}

func loadMetadataIfExists(workspace, metadataPath string) (*domain.Metadata, bool, error) {
	meta, err := LoadMetadata(workspace, metadataPath)

	if errors.Is(err, os.ErrNotExist) {
		return nil, false, errMetadataNotFound
	}

	if err != nil {
		return nil, false, fmt.Errorf("load metadata %q: %w", metadataPath, err)
	}

	return meta, true, nil
}

func loadPreviousLock(
	workspace string,
	cfg *config.Config,
	oldMeta *domain.Metadata,
) (lock *syncLock, target string, err error) {
	resolved := resolveOldLockPathAndTarget(resolveLockArgs{cfg: cfg, oldMeta: oldMeta})

	oldLock, err := LoadLock(workspace, resolved.lockPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, consts.Empty, fmt.Errorf("load lock %q: %w", resolved.lockPath, err)
	}

	oldTarget := resolved.target

	if oldLock != nil && oldTarget == consts.Empty {
		oldTarget = oldLock.Configuration.TargetFolder
	}

	return oldLock, oldTarget, nil
}

func resolveOldLockPathAndTarget(args resolveLockArgs) lockPathResult {
	oldLockPath := args.cfg.LockFilePath()

	if args.oldMeta == nil {
		return lockPathResult{lockPath: oldLockPath, target: consts.Empty}
	}

	if args.oldMeta.LockFile != consts.Empty {
		oldLockPath = args.oldMeta.LockFile
	}

	return lockPathResult{lockPath: oldLockPath, target: args.oldMeta.TargetFolder}
}

func discoverPreviousMetadata(workspace, currentMetadataPath string) (*domain.Metadata, error) {
	candidates, err := collectMetadataCandidates(workspace, currentMetadataPath)
	if err != nil {
		return nil, fmt.Errorf(errDiscoverPreviousMetadata, err)
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

	walker := metadataCandidateWalker(&metadataWalkerArgs{
		workspace:           workspace,
		currentMetadataPath: currentMetadataPath,
		candidates:          &candidates,
	})

	err := walkDir(workspace, walker)
	if err != nil {
		return nil, fmt.Errorf(errDiscoverPreviousMetadata, err)
	}

	slices.Sort(candidates)

	return candidates, nil
}

func metadataCandidateWalker(args *metadataWalkerArgs) func(string, os.DirEntry, error) error {
	return func(abs string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf(errWalk, abs, walkErr)
		}

		return processMetadataCandidate(args, abs, entry)
	}
}

func processMetadataCandidate(args *metadataWalkerArgs, abs string, entry os.DirEntry) error {
	rel, scan, err := previousMetadataCandidate(&metadataCandidateArgs{
		workspace:           args.workspace,
		currentMetadataPath: args.currentMetadataPath,
		abs:                 abs,
		entry:               entry,
	})
	if err == nil {
		recordMetadataCandidate(args.candidates, rel, scan)

		return nil
	}

	if errors.Is(err, filepath.SkipDir) {
		return filepath.SkipDir
	}

	return fmt.Errorf("previous metadata candidate: %w", err)
}

func recordMetadataCandidate(candidates *[]string, rel string, scan metadataScanResult) {
	if scan == metadataIsCandidate {
		*candidates = append(*candidates, rel)
	}
}

func loadFirstCandidate(workspace string, candidates []string) (*domain.Metadata, error) {
	meta, err := LoadMetadata(workspace, candidates[consts.IndexZero])
	if err != nil {
		return nil, fmt.Errorf("load previous metadata %q: %w", candidates[consts.IndexZero], err)
	}

	return meta, nil
}

func previousMetadataCandidate(args *metadataCandidateArgs) (string, metadataScanResult, error) {
	if args.entry.IsDir() {
		//nolint:wrapcheck // must return filepath.SkipDir sentinel unwrapped
		return handleDirEntry(args.entry)
	}

	rel, scan, err := metadataFileCandidate(args)
	if err != nil {
		return consts.Empty, metadataNotCandidate, fmt.Errorf("metadata file candidate: %w", err)
	}

	return rel, scan, nil
}

func metadataFileCandidate(args *metadataCandidateArgs) (string, metadataScanResult, error) {
	rel, err := relMetadataPath(args.workspace, args.abs)
	if err != nil {
		return consts.Empty, metadataNotCandidate, fmt.Errorf("relative metadata path: %w", err)
	}

	if isCurrentOrLegacyMetadataPath(rel, args.currentMetadataPath) {
		return consts.Empty, metadataNotCandidate, nil
	}

	if isTaskOtterMetadataPath(rel) {
		return rel, metadataIsCandidate, nil
	}

	return consts.Empty, metadataNotCandidate, nil
}

func relMetadataPath(workspace, abs string) (string, error) {
	rel, err := relPath(workspace, abs)
	if err != nil {
		return consts.Empty, fmt.Errorf("relative metadata path for %q: %w", abs, err)
	}

	return filepath.ToSlash(rel), nil
}

func handleDirEntry(entry os.DirEntry) (string, metadataScanResult, error) {
	if entry.Name() == ".git" {
		return consts.Empty, metadataNotCandidate, filepath.SkipDir
	}

	return consts.Empty, metadataNotCandidate, nil
}

func isCurrentOrLegacyMetadataPath(rel, currentMetadataPath string) bool {
	return rel == currentMetadataPath || rel == config.LegacyMetadataPath
}

func isTaskOtterMetadataPath(rel string) bool {
	return filepath.Base(rel) == storeMetadataFileName &&
		filepath.Base(filepath.Dir(rel)) == legacyMetadataDirName
}
