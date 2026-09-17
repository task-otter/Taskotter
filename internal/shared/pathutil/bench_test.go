// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil

import (
	"fmt"
	"strings"
	"testing"
)

const (
	benchmarkShallowDepth = 1
	benchmarkMediumDepth  = 5
	benchmarkDeepDepth    = 20
)

// BenchmarkValidateTargetFolder measures target-folder validation.
func BenchmarkValidateTargetFolder(b *testing.B) {
	for _, depth := range []int{benchmarkShallowDepth, benchmarkMediumDepth, benchmarkDeepDepth} {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			runValidateTargetFolderBenchmark(b, depth)
		})
	}
}

func runValidateTargetFolderBenchmark(b *testing.B, depth int) {
	b.Helper()

	workspace := b.TempDir()
	folder := benchmarkFolder(depth)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, err := ValidateTargetFolder(folder, workspace)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkFolder(depth int) string {
	var folderBuilder strings.Builder

	var written int

	written, err := folderBuilder.WriteString("taskfiles")
	if err != nil {
		return ""
	}

	if written == 0 {
		return ""
	}

	for i := range depth {
		_, err = fmt.Fprintf(&folderBuilder, "/module-%d", i)
		if err != nil {
			return ""
		}
	}

	return folderBuilder.String()
}
