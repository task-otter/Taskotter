// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

type (
	// Metadata records the synchronized target configuration.
	Metadata struct {
		// TargetFolder is the destination directory managed by TaskOtter.
		TargetFolder string `yaml:"target_folder"`
		// LockFile is the managed-state lock path relative to the workspace.
		LockFile string `yaml:"lock_file"`
		// ConfigurationHash identifies the configuration used for this synchronization.
		ConfigurationHash string `yaml:"configuration_hash"`
	}
	// SyncError is a synchronization error message.
	SyncError string
)
