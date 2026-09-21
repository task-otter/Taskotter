// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil_test

import (
	"os"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

func BenchmarkValidateRelativePath(b *testing.B) {
	rel := "taskfiles/eslint/node/pnpm/Taskfile.yml"

	b.ReportAllocs()

	for b.Loop() {
		_, err := pathutil.ValidateRelativePath("testfield", rel)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormalizeSlashes(b *testing.B) {
	p := `taskfiles\eslint\node\pnpm\Taskfile.yml`

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_ = pathutil.NormalizeSlashes(p)
	}
}

func BenchmarkValidateTargetFolder(b *testing.B) {
	tmpDir := b.TempDir()

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := pathutil.ValidateTargetFolder("taskfiles", tmpDir)

		if err != nil && !os.IsNotExist(err) {
			b.Fatal(err)
		}
	}
}
