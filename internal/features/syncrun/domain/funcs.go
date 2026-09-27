// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON encodes a resolved task using the GitHub Actions output keys.
func (task *ResolvedTask) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(map[string]string{
		"source_module":      task.SourceModule,
		"destination_module": task.DestinationModule,
		"path":               task.Path,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal resolved task: %w", err)
	}

	return data, nil
}
