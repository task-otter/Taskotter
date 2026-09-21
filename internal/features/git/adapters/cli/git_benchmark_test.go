// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/git/adapters/cli"
)

func createBenchmarkRepo(b *testing.B) string {
	b.Helper()

	root := b.TempDir()

	bareDir := filepath.Join(root, "bare.git")

	err := os.MkdirAll(bareDir, 0o755)
	if err != nil {
		b.Fatal(err)
	}

	cmdBare := exec.CommandContext(
		b.Context(),
		"git",
		"init",
		"--bare",
		"-b",
		testMainBranch,
	)

	cmdBare.Dir = bareDir

	err = cmdBare.Run()
	if err != nil {
		b.Fatal(err)
	}

	cloneDir := filepath.Join(root, "clone")

	err = os.MkdirAll(cloneDir, 0o755)
	if err != nil {
		b.Fatal(err)
	}

	cmdInit := exec.CommandContext(b.Context(), "git", "init", "-b", testMainBranch)

	cmdInit.Dir = cloneDir

	err = cmdInit.Run()
	if err != nil {
		b.Fatal(err)
	}

	configCmds := [][]string{
		{"config", "user.name", "Bench Test"},
		{"config", "user.email", "bench@test.local"},
		{"remote", "add", "origin", bareDir},
	}

	for _, args := range configCmds {
		c := exec.CommandContext(b.Context(), "git", args...)

		c.Dir = cloneDir

		err := c.Run()
		if err != nil {
			b.Fatal(err)
		}
	}

	dummyFile := filepath.Join(cloneDir, "README.md")

	err = os.WriteFile(dummyFile, []byte("# Test Repo\n"), 0o644)
	if err != nil {
		b.Fatal(err)
	}

	addCmd := exec.CommandContext(b.Context(), "git", "add", "README.md")

	addCmd.Dir = cloneDir

	err = addCmd.Run()
	if err != nil {
		b.Fatal(err)
	}

	commitCmd := exec.CommandContext(b.Context(), "git", "commit", "-m", "initial commit")

	commitCmd.Dir = cloneDir

	err = commitCmd.Run()
	if err != nil {
		b.Fatal(err)
	}

	pushCmd := exec.CommandContext(
		b.Context(),
		"git",
		"push",
		"-u",
		"origin",
		testMainBranch,
	)

	pushCmd.Dir = cloneDir

	err = pushCmd.Run()
	if err != nil {
		b.Fatal(err)
	}

	return cloneDir
}

func BenchmarkGitDefaultBranch(b *testing.B) {
	dir := createBenchmarkRepo(b)
	client := cli.NewClient(dir)

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := client.DefaultBranch(b.Context())
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGitHasUnrelatedChanges(b *testing.B) {
	dir := createBenchmarkRepo(b)
	client := cli.NewClient(dir)
	allowed := map[string]struct{}{"README.md": {}}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := client.HasUnrelatedChanges(b.Context(), allowed)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGitClient(b *testing.B) {
	b.Run("DefaultBranch", BenchmarkGitDefaultBranch)
	b.Run("HasUnrelatedChanges", BenchmarkGitHasUnrelatedChanges)
}
