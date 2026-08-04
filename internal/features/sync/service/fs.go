// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

// SetCopyFileToHookForTest installs a copy hook on plan for tests.
func SetCopyFileToHookForTest(plan *domain.Plan, hook func(string, *domain.FileEntry) error) {
	plan.CopyFileTo = hook
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := createTempFile(dir)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	err = finalizeTempFile(&finalizeTempArgs{tmp: tmp, data: data, mode: mode, path: path})
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

func finalizeTempFile(args *finalizeTempArgs) error {
	tmpPath := args.tmp.Name()
	cleanup := true

	defer cleanupTempFile(args.tmp, tmpPath, &cleanup)

	err := commitTempFile(args, tmpPath, &cleanup)
	if err != nil {
		return fmt.Errorf("commit temp file: %w", err)
	}

	return nil
}

func commitTempFile(args *finalizeTempArgs, tmpPath string, cleanup *bool) error {
	err := writeAndFinalizeTemp(args.tmp, args.data, args.mode)
	if err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	*cleanup = false

	err = renameTempFile(tmpPath, args.path)
	if err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func cleanupTempFile(tmp *os.File, tmpPath string, cleanup *bool) {
	if *cleanup {
		closeErr := tmp.Close()
		iox.Discard(closeErr)

		removeErr := os.Remove(tmpPath)
		iox.Discard(removeErr)
	}
}

func writeAndFinalizeTemp(tmp *os.File, data []byte, mode os.FileMode) error {
	err := iox.WriteFull(tmp, data)
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

func copyFileTo(path string, entry *domain.FileEntry) error {
	err := writeFileAtomic(path, entry.Data, entry.Mode)
	if err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}

	return nil
}

// CopyFile copies rel under root to dst with the given mode.
func CopyFile(args *copyFileArgs) error {
	data, err := readRelativeFile(args.root, args.rel)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	err = writeCopiedFile(args.dst, data, args.mode)
	if err != nil {
		return fmt.Errorf("write copied file: %w", err)
	}

	return nil
}

func readRelativeFile(root, rel string) ([]byte, error) {
	source, err := pathutil.OpenRelativeFile(root, rel)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", rel, err)
	}

	defer func() {
		closeErr := source.Close()
		iox.Discard(closeErr)
	}()

	data, err := io.ReadAll(source)
	if err != nil {
		return nil, fmt.Errorf(errFmtReadQuoted, rel, err)
	}

	return data, nil
}

func writeCopiedFile(dst string, data []byte, mode os.FileMode) error {
	err := writeFileAtomic(dst, data, mode)
	if err != nil {
		return fmt.Errorf(errFmtWriteQuoted, dst, err)
	}

	return nil
}

func sortedModuleRecords(requested map[string]moduleRecord, deps []moduleRecord) []moduleRecord {
	tasks := make([]string, consts.IndexZero, len(requested))

	for task := range requested {
		tasks = append(tasks, task)
	}

	slices.Sort(tasks)

	out := make([]moduleRecord, consts.IndexZero, len(requested)+len(deps))

	for i := range tasks {
		out = append(out, requested[tasks[i]])
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
