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
	got := marshalTask(t, task)

	assertJSONField(t, got, jsonFieldAssertion{jsonKeySourceModule, task.SourceModule})
	assertJSONField(t, got, jsonFieldAssertion{jsonKeyDestinationModule, task.DestinationModule})
	assertJSONField(t, got, jsonFieldAssertion{jsonKeyPath, task.Path})
}

func marshalTask(t *testing.T, task *ResolvedTask) map[string]string {
	t.Helper()

	data, err := task.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var got map[string]string

	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	return got
}

type jsonFieldAssertion struct {
	name, want string
}

func assertJSONField(t *testing.T, got map[string]string, assertion jsonFieldAssertion) {
	t.Helper()

	if got[assertion.name] != assertion.want {
		t.Fatalf("%s = %q", assertion.name, got[assertion.name])
	}
}
