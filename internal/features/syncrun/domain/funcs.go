// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"encoding/json"
)

// MarshalJSON encodes the resolved task using the public output keys.
func (task *ResolvedTask) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"source_module":      task.SourceModule,
		"destination_module": task.DestinationModule,
		"path":               task.Path,
	})
}
