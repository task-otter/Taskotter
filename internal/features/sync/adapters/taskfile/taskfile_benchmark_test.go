// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
)

func BenchmarkRewriteIncludes(b *testing.B) {
	input := rewriteIncludesInput()
	mapping := map[string]string{
		"../../../pnpm/Taskfile.yml": "../pnpm/Taskfile.yml",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := taskfile.RewriteIncludes(input, mapping, "eslint/node/pnpm")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUpdateRootTaskfile(b *testing.B) {
	template := taskfile.NewRootTemplate()
	input := goOnlyRootInput()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := taskfile.UpdateRootTaskfile(template, input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRewriteIncludesSpans(b *testing.B) {
	input := []byte("version: \"3\"\nincludes:\n  pnpm:\n    taskfile: \"../../../pnpm/Taskfile.yml\"\n  eslint:\n    taskfile: \"../../../eslint/Taskfile.yml\"\n  prettier:\n    taskfile: \"../../prettier/Taskfile.yml\"\n")
	mapping := map[string]string{
		"../../../pnpm/Taskfile.yml":   "../pnpm/Taskfile.yml",
		"../../../eslint/Taskfile.yml": "../eslint/Taskfile.yml",
		"../../prettier/Taskfile.yml":  "../prettier/Taskfile.yml",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := taskfile.RewriteIncludes(input, mapping, "eslint/node/pnpm")
		if err != nil {
			b.Fatal(err)
		}
	}
}
