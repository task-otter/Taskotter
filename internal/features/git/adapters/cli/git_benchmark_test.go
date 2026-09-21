// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli_test

import (
	"context"
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
	if err := os.MkdirAll(bareDir, 0755); err != nil {
		b.Fatal(err)
	}
	cmdBare := exec.CommandContext(context.Background(), "git", "init", "--bare", "-b", testMainBranch)
	cmdBare.Dir = bareDir
	if err := cmdBare.Run(); err != nil {
		b.Fatal(err)
	}

	cloneDir := filepath.Join(root, "clone")
	if err := os.MkdirAll(cloneDir, 0755); err != nil {
		b.Fatal(err)
	}

	cmdInit := exec.CommandContext(context.Background(), "git", "init", "-b", testMainBranch)
	cmdInit.Dir = cloneDir
	if err := cmdInit.Run(); err != nil {
		b.Fatal(err)
	}

	configCmds := [][]string{
		{"config", "user.name", "Bench Test"},
		{"config", "user.email", "bench@test.local"},
		{"remote", "add", "origin", bareDir},
	}
	for _, args := range configCmds {
		c := exec.CommandContext(context.Background(), "git", args...)
		c.Dir = cloneDir
		if err := c.Run(); err != nil {
			b.Fatal(err)
		}
	}

	dummyFile := filepath.Join(cloneDir, "README.md")
	if err := os.WriteFile(dummyFile, []byte("# Test Repo\n"), 0644); err != nil {
		b.Fatal(err)
	}

	addCmd := exec.CommandContext(context.Background(), "git", "add", "README.md")
	addCmd.Dir = cloneDir
	if err := addCmd.Run(); err != nil {
		b.Fatal(err)
	}

	commitCmd := exec.CommandContext(context.Background(), "git", "commit", "-m", "initial commit")
	commitCmd.Dir = cloneDir
	if err := commitCmd.Run(); err != nil {
		b.Fatal(err)
	}

	pushCmd := exec.CommandContext(context.Background(), "git", "push", "-u", "origin", testMainBranch)
	pushCmd.Dir = cloneDir
	if err := pushCmd.Run(); err != nil {
		b.Fatal(err)
	}

	return cloneDir
}

func BenchmarkGitDefaultBranch(b *testing.B) {
	dir := createBenchmarkRepo(b)
	client := cli.NewClient(dir)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := client.DefaultBranch(context.Background())
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
	for i := 0; i < b.N; i++ {
		_, err := client.HasUnrelatedChanges(context.Background(), allowed)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGitClient(b *testing.B) {
	b.Run("DefaultBranch", BenchmarkGitDefaultBranch)
	b.Run("HasUnrelatedChanges", BenchmarkGitHasUnrelatedChanges)
}
