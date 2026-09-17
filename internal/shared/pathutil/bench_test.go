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
	benchmarkEmptyLength  = 0
	benchmarkEmptyString  = ""
	benchmarkTaskfilesDir = "taskfiles"
)

// BenchmarkValidateTargetFolder measures target-folder validation.
func BenchmarkValidateTargetFolder(b *testing.B) {
	depths := []int{benchmarkShallowDepth, benchmarkMediumDepth, benchmarkDeepDepth}

	for index := range depths {
		depth := depths[index]
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

//nolint:funlen,revive // benchmark input construction is clearer as one operation
func benchmarkFolder(depth int) string {
	var folderBuilder strings.Builder

	var written int

	written, err := folderBuilder.WriteString(benchmarkTaskfilesDir)
	if err != nil {
		return benchmarkEmptyString
	}

	if written == benchmarkEmptyLength {
		return benchmarkEmptyString
	}

	err = appendBenchmarkModules(&folderBuilder, depth)
	if err != nil {
		return benchmarkEmptyString
	}

	return folderBuilder.String()
}

func appendBenchmarkModules(builder *strings.Builder, depth int) error {
	for i := range depth {
		_, err := fmt.Fprintf(builder, "/module-%d", i)
		if err != nil {
			return fmt.Errorf("append benchmark module: %w", err)
		}
	}

	return nil
}
