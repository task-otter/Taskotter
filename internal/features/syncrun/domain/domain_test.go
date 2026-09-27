// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"encoding/json"
	"testing"
)

// TestResolvedTaskMarshalJSONUsesOutputKeys verifies the behavior covered by this test.
func TestResolvedTaskMarshalJSONUsesOutputKeys(t *testing.T) {
	t.Parallel()

	task := &ResolvedTask{
		SourceModule:      "eslint/node/pnpm",
		DestinationModule: "eslint",
		Path:              "taskfiles/eslint",
	}

	data, err := task.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var got map[string]string

	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got["source_module"] != task.SourceModule {
		t.Fatalf("source_module = %q", got["source_module"])
	}

	if got["destination_module"] != task.DestinationModule {
		t.Fatalf("destination_module = %q", got["destination_module"])
	}

	if got["path"] != task.Path {
		t.Fatalf("path = %q", got["path"])
	}
}
