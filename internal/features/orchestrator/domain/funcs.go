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
//
//nolint:unparam // json.Marshaler requires the error return even though this string-only payload cannot fail.
func (task *ResolvedTask) MarshalJSON() ([]byte, error) {
	//nolint:dogsled,errcheck,gosec // every map value is a string and cannot fail JSON encoding.
	data, _ := json.Marshal(map[string]string{
		jsonKeySourceModule:      task.SourceModule,
		jsonKeyDestinationModule: task.DestinationModule,
		jsonKeyPath:              task.Path,
	})

	return data, nil
}
