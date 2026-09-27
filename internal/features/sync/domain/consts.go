// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	YAMLMappingPairKeyValue = consts.IndexTwo

	errDecode                = "decode %q: %w"
	errUnmarshalMetadataFmt  = "unmarshal metadata: %w"
	yamlKeyConfigurationHash = "configuration_hash"
	yamlKeyLockFile          = "lock_file"
	yamlKeyTargetFolder      = "target_folder"

	YAMLKeyExportedTasks = "exported_tasks"

	YAMLKeyModule = "module"

	YAMLKeySchema = "schema"

	YAMLKeyTaskfile = "taskfile"

	YAMLKeyVariants = "variants"
)
