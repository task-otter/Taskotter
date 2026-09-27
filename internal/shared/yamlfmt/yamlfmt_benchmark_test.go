// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
)

// BenchmarkMarshal measures performance.
func BenchmarkMarshal(b *testing.B) {
	data := benchmarkMarshalData()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := yamlfmt.Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkMarshalData() map[string]any {
	return map[string]any{
		fixtureVersionKey: fixtureVersion,
		fixtureVarsKey: map[string]string{
			fixtureGoVersionKey: fixtureGoVersion,
			"NODE_ENV":          "production",
		},
		"includes": map[string]any{
			"go":     "taskfiles/go/Taskfile.yml",
			"eslint": "taskfiles/eslint/Taskfile.yml",
		},
	}
}
