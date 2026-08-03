// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package syncer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
)

// SetCopyFileToHookForTest installs a copy hook on plan for tests.
func SetCopyFileToHookForTest(plan *Plan, hook func(string, FileEntry) error) {
	plan.copyFileTo = hook
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := createTempFile(dir)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	err = finalizeTempFile(tmp, data, mode, path)
	if err != nil {
		return fmt.Errorf("finalize temp file: %w", err)
	}

	return nil
}

func createTempFile(dir string) (*os.File, error) {
	err := os.MkdirAll(dir, dirModePerm)
	if err != nil {
		return nil, fmt.Errorf("create directory %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".taskotter-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file in %q: %w", dir, err)
	}

	return tmp, nil
}

func finalizeTempFile(tmp *os.File, data []byte, mode os.FileMode, path string) error {
	tmpPath := tmp.Name()
	cleanup := true

	defer cleanupTempFile(tmp, tmpPath, &cleanup)

	err := writeAndFinalizeTemp(tmp, data, mode)
	if err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	cleanup = false

	err = renameTempFile(tmpPath, path)
	if err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func cleanupTempFile(tmp *os.File, tmpPath string, cleanup *bool) {
	if *cleanup {
		_ = tmp.Close()        //nolint:errcheck // best-effort cleanup
		_ = os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
	}
}

func writeAndFinalizeTemp(tmp *os.File, data []byte, mode os.FileMode) error {
	_, err := tmp.Write(data)
	if err != nil {
		return fmt.Errorf("write temp file %q: %w", tmp.Name(), err)
	}

	err = tmp.Chmod(mode)
	if err != nil {
		return fmt.Errorf("chmod temp file %q: %w", tmp.Name(), err)
	}

	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("close temp file %q: %w", tmp.Name(), err)
	}

	return nil
}

func renameTempFile(tmpPath, path string) error {
	err := os.Rename(tmpPath, path)
	if err != nil {
		return fmt.Errorf("rename temp file to %q: %w", path, err)
	}

	return nil
}

func copyFileTo(path string, entry FileEntry) error {
	return writeFileAtomic(path, entry.Data, entry.Mode)
}

// CopyFile copies rel under root to dst with the given mode.
func CopyFile(root, rel, dst string, mode os.FileMode) error {
	source, err := pathutil.OpenRelativeFile(root, rel)
	if err != nil {
		return fmt.Errorf("open %q: %w", rel, err)
	}

	defer func() { _ = source.Close() }() //nolint:errcheck // read-only close

	data, err := io.ReadAll(source)
	if err != nil {
		return fmt.Errorf("read %q: %w", rel, err)
	}

	err = writeFileAtomic(dst, data, mode)
	if err != nil {
		return fmt.Errorf("write %q: %w", dst, err)
	}

	return nil
}

func sortedModuleRecords(requested map[string]ModuleRecord, deps []ModuleRecord) []ModuleRecord {
	tasks := make([]string, consts.IndexZero, len(requested))

	for task := range requested {
		tasks = append(tasks, task)
	}

	sort.Strings(tasks)

	out := make([]ModuleRecord, consts.IndexZero, len(requested)+len(deps))

	for _, task := range tasks {
		out = append(out, requested[task])
	}

	return append(out, deps...)
}

func preserveMode(mode os.FileMode) os.FileMode {
	perm := mode.Perm()

	if perm&consts.FilePerm111 != consts.IndexZero {
		return perm
	}

	return fileModeRegular
}
