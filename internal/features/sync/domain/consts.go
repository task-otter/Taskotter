// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	// YAMLMappingPairKeyValue is the stride between key and value nodes in a YAML mapping.
	YAMLMappingPairKeyValue = consts.IndexTwo

	errDecode                = "decode %q: %w"
	errUnmarshalMetadataFmt  = "unmarshal metadata: %w"
	yamlKeyConfigurationHash = "configuration_hash"
	yamlKeyLockFile          = "lock_file"
	yamlKeyTargetFolder      = "target_folder"

	// YAMLKeyExportedTasks is the store metadata key for exported tasks.
	YAMLKeyExportedTasks = "exported_tasks"

	// YAMLKeyModule is the store metadata key for the module name.
	YAMLKeyModule = "module"

	// YAMLKeySchema is the store metadata key for the schema version.
	YAMLKeySchema = "schema"

	// YAMLKeyTaskfile is the store metadata key for the taskfile path.
	YAMLKeyTaskfile = "taskfile"

	// YAMLKeyVariants is the store metadata key for module variants.
	YAMLKeyVariants = "variants"
)
