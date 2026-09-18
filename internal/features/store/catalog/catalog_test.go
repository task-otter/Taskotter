// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverModulesAndVariants(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	module := filepath.Join(root, taskfilesDir, "lint")
	variant := filepath.Join(module, "node", "pnpm")

	if err := os.MkdirAll(variant, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(module, "Taskfile.yml"),
		[]byte("version: '3'\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := got["lint"]; !ok {
		t.Fatalf("modules = %#v", got)
	}

	if _, ok := got["lint/node/pnpm"]; ok {
		t.Fatalf("variant registered as module: %#v", got)
	}
}

func TestDiscoverMissingRoot(t *testing.T) {
	t.Parallel()

	if _, err := Discover(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing catalog root accepted")
	}
}

func TestDiscoverReportsUnreadableChild(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	taskfiles := filepath.Join(root, taskfilesDir)
	child := filepath.Join(taskfiles, "broken")

	err := os.MkdirAll(child, 0o750)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chmod(child, 0)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Discover(root); err == nil {
		t.Fatal("unreadable child accepted")
	}
}
