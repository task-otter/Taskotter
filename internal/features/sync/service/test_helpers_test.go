// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

func writeRootTaskfile(tb testing.TB, workspace string) {
	tb.Helper()
	writeFileWithDir(tb, filepath.Join(workspace, testTaskfileName), []byte(benchRootBody))
}

func writeFileWithDir(tb testing.TB, path string, data []byte) {
	tb.Helper()

	err := os.MkdirAll(filepath.Dir(path), consts.FilePerm755)
	if err != nil {
		tb.Fatal(err)
	}

	err = os.WriteFile(path, data, consts.FilePerm644)
	if err != nil {
		tb.Fatal(err)
	}
}
