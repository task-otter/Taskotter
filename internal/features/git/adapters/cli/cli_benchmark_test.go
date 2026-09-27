// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/git/adapters/cli"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

func createBenchmarkRepo(b *testing.B) string {
	b.Helper()

	root := b.TempDir()
	bareDir := filepath.Join(root, testBareRepoDir)
	createBareRepo(b, bareDir)

	cloneDir := filepath.Join(root, "clone")
	createCloneRepo(b, cloneDir)
	configureClone(b, cloneDir, bareDir)
	commitReadme(b, cloneDir)
	pushClone(b, cloneDir)

	return cloneDir
}

func createBareRepo(b *testing.B, dir string) {
	b.Helper()

	makeDir(b, dir)
	benchRunGit(b, dir, gitSubcommandInit, flagBare, flagB, testMainBranch)
}

func createCloneRepo(b *testing.B, dir string) {
	b.Helper()

	makeDir(b, dir)
	benchRunGit(b, dir, gitSubcommandInit, flagB, testMainBranch)
}

func configureClone(b *testing.B, dir, bareDir string) {
	b.Helper()

	commands := [][]string{
		{gitCmdConfig, "user.name", "Bench Test"},
		{gitCmdConfig, "user.email", "bench@test.local"},
		{gitCmdRemote, gitCmdAdd, consts.GitOrigin, bareDir},
	}

	for i := range commands {
		benchRunGit(b, dir, commands[i]...)
	}
}

func commitReadme(b *testing.B, dir string) {
	b.Helper()

	readme := filepath.Join(dir, consts.ReadmeMD)

	err := os.WriteFile(readme, []byte("# Test Repo\n"), consts.FilePerm644)
	if err != nil {
		b.Fatal(err)
	}

	benchRunGit(b, dir, gitCmdAdd, consts.ReadmeMD)
	benchRunGit(b, dir, gitCmdCommit, flagM, "initial commit")
}

func pushClone(b *testing.B, dir string) {
	b.Helper()

	benchRunGit(b, dir, gitCmdPush, flagSetUpstream, consts.GitOrigin, testMainBranch)
}

func makeDir(b *testing.B, dir string) {
	b.Helper()

	err := os.MkdirAll(dir, consts.FilePerm755)
	if err != nil {
		b.Fatal(err)
	}
}

func benchRunGit(b *testing.B, dir string, args ...string) {
	b.Helper()

	cmd := exec.CommandContext(b.Context(), gitBinaryName, args...)

	cmd.Dir = dir

	err := cmd.Run()
	if err != nil {
		b.Fatal(err)
	}
}

// BenchmarkGitDefaultBranch measures performance.
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

// BenchmarkGitHasUnrelatedChanges measures performance.
func BenchmarkGitHasUnrelatedChanges(b *testing.B) {
	dir := createBenchmarkRepo(b)
	client := cli.NewClient(dir)
	allowed := map[string]struct{}{consts.ReadmeMD: {}}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := client.HasUnrelatedChanges(b.Context(), allowed)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGitClient measures performance.
func BenchmarkGitClient(b *testing.B) {
	b.Run("DefaultBranch", BenchmarkGitDefaultBranch)
	b.Run("HasUnrelatedChanges", BenchmarkGitHasUnrelatedChanges)
}
