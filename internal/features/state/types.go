// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/state/managed"
)

type (
	Metadata struct {
		TargetFolder      string `yaml:"target_folder"`
		LockFile          string `yaml:"lock_file"`
		ConfigurationHash string `yaml:"configuration_hash"`
	}
	LockFile          = lockmodel.LockFile
	LockSource        = lockmodel.LockSource
	LockConfiguration = lockmodel.LockConfiguration
	ModuleRecord      = lockmodel.ModuleRecord
	OrderedRequested  = lockmodel.OrderedRequested
	ManagedFile       = managed.File
	SyncError         string

	yamlDecodeTarget struct {
		Out any
		Key string
	}

	// Previous contains the persisted state discovered before a sync run.
	Previous struct {
		Metadata        *Metadata
		Lock            *LockFile
		MetadataPath    string
		LockPath        string
		OldTargetFolder string
	}
)
