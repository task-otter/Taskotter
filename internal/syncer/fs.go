package syncer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/task-otter/Taskotter/internal/pathutil"
)

// CopyFileToHook replaces copyFileTo during tests when non-nil.
//
//nolint:gochecknoglobals // test hook
var CopyFileToHook func(path string, entry FileEntry) error

//nolint:gochecknoglobals // protects CopyFileToHook reads during concurrent copyFileTo calls
var copyFileHookMu sync.RWMutex

// SetCopyFileToHookForTest installs a test hook. Pair with ClearCopyFileToHookForTest in t.Cleanup.
func SetCopyFileToHookForTest(hook func(path string, entry FileEntry) error) {
	copyFileHookMu.Lock()
	CopyFileToHook = hook
	copyFileHookMu.Unlock()
}

// ClearCopyFileToHookForTest removes the test hook installed by SetCopyFileToHookForTest.
func ClearCopyFileToHookForTest() {
	copyFileHookMu.Lock()
	CopyFileToHook = nil
	copyFileHookMu.Unlock()
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)

	err := os.MkdirAll(dir, dirModePerm)
	if err != nil {
		return fmt.Errorf("create directory %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".taskotter-*")
	if err != nil {
		return fmt.Errorf("create temp file in %q: %w", dir, err)
	}

	tmpPath := tmp.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	_, err = tmp.Write(data)
	if err != nil {
		return fmt.Errorf("write temp file %q: %w", tmpPath, err)
	}

	err = tmp.Chmod(mode)
	if err != nil {
		return fmt.Errorf("chmod temp file %q: %w", tmpPath, err)
	}

	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("close temp file %q: %w", tmpPath, err)
	}

	cleanup = false

	err = os.Rename(tmpPath, path)
	if err != nil {
		return fmt.Errorf("rename temp file to %q: %w", path, err)
	}

	return nil
}

func copyFileTo(path string, entry FileEntry) error {
	copyFileHookMu.RLock()

	hook := CopyFileToHook

	copyFileHookMu.RUnlock()

	if hook != nil {
		return hook(path, entry)
	}

	return writeFileAtomic(path, entry.Data, entry.Mode)
}

// CopyFile copies rel under root to dst with the given mode.
func CopyFile(root, rel, dst string, mode os.FileMode) error {
	source, err := pathutil.OpenRelativeFile(root, rel)
	if err != nil {
		return fmt.Errorf("open %q: %w", rel, err)
	}
	defer func() { _ = source.Close() }()

	data, err := io.ReadAll(source)
	if err != nil {
		return fmt.Errorf("read %q: %w", rel, err)
	}

	return writeFileAtomic(dst, data, mode)
}

func sortedModuleRecords(requested map[string]ModuleRecord, deps []ModuleRecord) []ModuleRecord {
	tasks := make([]string, 0, len(requested))
	for task := range requested {
		tasks = append(tasks, task)
	}

	sort.Strings(tasks)

	out := make([]ModuleRecord, 0, len(requested)+len(deps))
	for _, task := range tasks {
		out = append(out, requested[task])
	}

	return append(out, deps...)
}

func preserveMode(mode os.FileMode) os.FileMode {
	perm := mode.Perm()
	if perm&0o111 != 0 {
		return perm
	}

	return fileModeRegular
}
