// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package lockmodel

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	errDecode                   = "decode %q: %w"
	errUnmarshalModuleRecordFmt = "unmarshal module record: %w"
	yamlLabelModuleRecord       = "module record"
	yamlKeyConfiguration        = "configuration"
	yamlKeyDefaultBranch        = "default_branch"
	yamlKeyDependencies         = "dependencies"
	yamlKeyDestinationModule    = "destination_module"
	yamlKeyGeneratedRootTasks   = "generated_root_tasks"
	yamlKeyIncludesDoc          = "includes_doc"
	yamlKeyManagedFiles         = "managed_files"
	yamlKeyNodePackageManager   = "node_package_manager"
	yamlKeyPath                 = "path"
	yamlKeyRepository           = "repository"
	yamlKeyRequested            = "requested"
	yamlKeyRequestedVersion     = "requested_version"
	yamlKeyResolvedCommit       = "resolved_commit"
	yamlKeyResolvedModules      = "resolved_modules"
	yamlKeySHA256               = "sha256"
	yamlKeySource               = "source"
	yamlKeySourceModule         = "source_module"
	yamlKeySourcePath           = "source_path"
	yamlKeySourceRef            = "source_ref"
	yamlKeySyncRoot             = "sync_root"
	yamlKeyTargetFolder         = "target_folder"
	yamlKeyTasks                = "tasks"
	yamlMappingPairKeyValue     = consts.IndexTwo
)
