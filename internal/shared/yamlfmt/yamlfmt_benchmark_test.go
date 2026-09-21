// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
)

func BenchmarkMarshal(b *testing.B) {
	data := map[string]any{
		"version": "3",
		"vars": map[string]string{
			"GO_VERSION": "1.26.5",
			"NODE_ENV":   "production",
		},
		"includes": map[string]any{
			"go":     "taskfiles/go/Taskfile.yml",
			"eslint": "taskfiles/eslint/Taskfile.yml",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := yamlfmt.Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
