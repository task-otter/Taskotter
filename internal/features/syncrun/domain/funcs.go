// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"encoding/json"
)

const (
	jsonKeySourceModule      = "source_module"
	jsonKeyDestinationModule = "destination_module"
	jsonKeyPath              = "path"
)

// MarshalJSON encodes the resolved task using the public output keys.
func (task *ResolvedTask) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		jsonKeySourceModule:      task.SourceModule,
		jsonKeyDestinationModule: task.DestinationModule,
		jsonKeyPath:              task.Path,
	})
}
