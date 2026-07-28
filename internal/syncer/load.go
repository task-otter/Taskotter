package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/pathutil"
	"gopkg.in/yaml.v3"
)

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
	oldMeta, err := LoadMetadata(workspace, cfg.MetadataPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, "", err
	}
	if oldMeta == nil && cfg.MetadataPath() != config.LegacyMetadataPath {
		oldMeta, err = LoadMetadata(workspace, config.LegacyMetadataPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, "", err
		}
	}
	if oldMeta == nil {
		oldMeta, err = discoverPreviousMetadata(workspace, cfg.MetadataPath())
		if err != nil {
			return nil, nil, "", err
		}
	}

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
		return nil, nil, "", err
	}

	if oldLock != nil && oldTarget == "" {
		oldTarget = oldLock.Configuration.TargetFolder
	}

	return oldLock, oldMeta, oldTarget, nil
}

func discoverPreviousMetadata(workspace, currentMetadataPath string) (*Metadata, error) {
	var candidates []string

	err := filepath.WalkDir(workspace, func(abs string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		rel, relErr := filepath.Rel(workspace, abs)
		if relErr != nil {
			return relErr
		}

		rel = filepath.ToSlash(rel)
		if rel == currentMetadataPath || filepath.ToSlash(rel) == config.LegacyMetadataPath {
			return nil
		}

		if filepath.Base(rel) == "metadata.yml" &&
			filepath.Base(filepath.Dir(rel)) == ".taskotter" {
			candidates = append(candidates, rel)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover previous metadata: %w", err)
	}

	sort.Strings(candidates)
	for _, candidate := range candidates {
		meta, loadErr := LoadMetadata(workspace, candidate)
		if loadErr != nil {
			return nil, loadErr
		}

		return meta, nil
	}

	return nil, nil
}
