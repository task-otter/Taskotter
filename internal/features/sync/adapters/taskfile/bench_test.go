// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"fmt"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/ports"
)

const (
	benchmarkSmallSize   = 2
	benchmarkMediumSize  = 10
	benchmarkLargeSize   = 50
	benchmarkFirstIndex  = 0
	benchmarkEmptyLength = 0
)

// BenchmarkRewriteIncludes measures include rewriting.
func BenchmarkRewriteIncludes(b *testing.B) {
	for _, size := range []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize} {
		b.Run(fmt.Sprintf("includes_%d", size), func(b *testing.B) {
			runRewriteIncludesBenchmark(b, size)
		})
	}
}

func runRewriteIncludesBenchmark(b *testing.B, size int) {
	b.Helper()

	content := []byte("version: \"3\"\nincludes:\n")
	mapping := make(map[string]string, size)

	for i := range size {
		name := fmt.Sprintf("module-%d", i+benchmarkFirstIndex)

		content = append(
			content,
			[]byte(fmt.Sprintf("  %s:\n    taskfile: ../%s/Taskfile.yml\n", name, name))...)
		mapping[name] = "taskfiles/" + name
	}

	b.ReportAllocs()
	b.ResetTimer()
	runRewriteIncludesIterations(b, content, mapping)
}

func runRewriteIncludesIterations(b *testing.B, content []byte, mapping map[string]string) {
	b.Helper()

	for range b.N {
		result, err := RewriteIncludes(content, mapping, "taskfiles")

		if err != nil || len(result) == benchmarkEmptyLength {
			b.Fatalf("rewrite includes: %v", err)
		}
	}
}

// BenchmarkUpdateRootTaskfile measures root taskfile updates.
func BenchmarkUpdateRootTaskfile(b *testing.B) {
	for _, size := range []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize} {
		b.Run(fmt.Sprintf("modules_%d", size), func(b *testing.B) {
			runUpdateRootTaskfileBenchmark(b, size)
		})
	}
}

func runUpdateRootTaskfileBenchmark(b *testing.B, size int) {
	b.Helper()

	inputValue := rootUpdateInputForBenchmark(size)
	input := &inputValue
	content := NewRootTemplate()

	b.ReportAllocs()
	b.ResetTimer()
	runUpdateRootTaskfileIterations(b, content, input)
}

func runUpdateRootTaskfileIterations(b *testing.B, content []byte, input *ports.RootUpdateInput) {
	b.Helper()

	for range b.N {
		result, err := UpdateRootTaskfile(content, input)

		if err != nil || len(result) == benchmarkEmptyLength {
			b.Fatalf("update root taskfile: %v", err)
		}
	}
}

func rootUpdateInputForBenchmark(size int) ports.RootUpdateInput {
	input := ports.RootUpdateInput{
		TargetFolder:    "taskfiles",
		RootTaskfileDir: ".",
		DestByTask:      make(map[string]string, size),
		ModuleTaskfiles: make(map[string][]byte, size),
	}

	for i := range size {
		name := fmt.Sprintf("module-%d", i+benchmarkFirstIndex)

		input.Tasks = append(input.Tasks, name)
		input.ManagedTasks = append(input.ManagedTasks, name)
		input.DestByTask[name] = name
		input.ModuleTaskfiles[name] = []byte("version: \"3\"\n")
	}

	return input
}
