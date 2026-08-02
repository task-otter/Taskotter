// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"fmt"
	"sort"

	yaml "go.yaml.in/yaml/v3"
)

type orderedRequested map[string]ModuleRecord

func (m orderedRequested) MarshalYAML() (any, error) {
	if len(m) == 0 {
		return map[string]ModuleRecord{}, nil
	}

	keys := make([]string, 0, len(m))

	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	out := make(map[string]ModuleRecord, len(m))

	for _, k := range keys {
		out[k] = m[k]
	}

	return out, nil
}

func (m *orderedRequested) UnmarshalYAML(value *yaml.Node) error {
	var raw map[string]ModuleRecord

	err := value.Decode(&raw)
	if err != nil {
		return fmt.Errorf("decode ordered requested modules: %w", err)
	}

	*m = orderedRequested(raw)

	return nil
}
