// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package managed

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	errDecode                = "decode %q: %w"
	errUnmarshalManagedFmt   = "unmarshal managed file: %w"
	yamlKeyDestinationModule = "destination_module"
	yamlKeyPath              = "path"
	yamlKeySHA256            = "sha256"
	yamlKeySourceModule      = "source_module"
	yamlKeySourcePath        = "source_path"
	yamlMappingPairKeyValue  = consts.IndexTwo
)
