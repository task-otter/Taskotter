// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/state/managed"
)

type (
	// Metadata records the synchronized target configuration.
	Metadata struct {
		TargetFolder      string `yaml:"target_folder"`
		LockFile          string `yaml:"lock_file"`
		ConfigurationHash string `yaml:"configuration_hash"`
	}
	// LockFile records managed synchronization state.
	LockFile = lockmodel.LockFile
	// LockSource identifies the source store.
	LockSource = lockmodel.LockSource
	// LockConfiguration records sync configuration.
	LockConfiguration = lockmodel.LockConfiguration
	// ModuleRecord records a synchronized module.
	ModuleRecord = lockmodel.ModuleRecord
	// OrderedRequested records requested modules in order.
	OrderedRequested = lockmodel.OrderedRequested
	// ManagedFile records a managed file.
	ManagedFile = managed.File
	// SyncError is a synchronization error message.
	SyncError string

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
