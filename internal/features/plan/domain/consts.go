// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	// YAMLMappingPairKeyValue is the YAML key used by the synchronization metadata.
	YAMLMappingPairKeyValue = consts.IndexTwo

	errDecode                = "decode %q: %w"
	errUnmarshalMetadataFmt  = "unmarshal metadata: %w"
	yamlKeyConfigurationHash = "configuration_hash"
	yamlKeyLockFile          = "lock_file"
	yamlKeyTargetFolder      = "target_folder"

	// YAMLKeyExportedTasks is the YAML key used by the synchronization metadata.
	YAMLKeyExportedTasks = "exported_tasks"

	// YAMLKeyModule is the YAML key used by the synchronization metadata.
	YAMLKeyModule = "module"

	// YAMLKeySchema is the YAML key used by the synchronization metadata.
	YAMLKeySchema = "schema"

	// YAMLKeyTaskfile is the YAML key used by the synchronization metadata.
	YAMLKeyTaskfile = "taskfile"

	// YAMLKeyVariants is the YAML key used by the synchronization metadata.
	YAMLKeyVariants = "variants"
)
