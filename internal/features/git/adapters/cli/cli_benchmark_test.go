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

	err := os.MkdirAll(bareDir, consts.FilePerm755)
	if err != nil {
		b.Fatal(err)
	}

	cmdBare := exec.CommandContext(
		b.Context(),
		gitBinaryName,
		gitSubcommandInit,
		flagBare,
		flagB,
		testMainBranch,
	)

	cmdBare.Dir = bareDir

	err = cmdBare.Run()
	if err != nil {
		b.Fatal(err)
	}

	cloneDir := filepath.Join(root, "clone")

	err = os.MkdirAll(cloneDir, consts.FilePerm755)
	if err != nil {
		b.Fatal(err)
	}

	cmdInit := exec.CommandContext(
		b.Context(),
		gitBinaryName,
		gitSubcommandInit,
		flagB,
		testMainBranch,
	)

	cmdInit.Dir = cloneDir

	err = cmdInit.Run()
	if err != nil {
		b.Fatal(err)
	}

	configCmds := [][]string{
		{gitCmdConfig, "user.name", "Bench Test"},
		{gitCmdConfig, "user.email", "bench@test.local"},
		{gitCmdRemote, gitCmdAdd, consts.GitOrigin, bareDir},
	}

	for _, args := range configCmds {
		c := exec.CommandContext(b.Context(), gitBinaryName, args...)

		c.Dir = cloneDir

		err := c.Run()
		if err != nil {
			b.Fatal(err)
		}
	}

	dummyFile := filepath.Join(cloneDir, consts.ReadmeMD)

	err = os.WriteFile(dummyFile, []byte("# Test Repo\n"), consts.FilePerm644)
	if err != nil {
		b.Fatal(err)
	}

	addCmd := exec.CommandContext(b.Context(), gitBinaryName, gitCmdAdd, consts.ReadmeMD)

	addCmd.Dir = cloneDir

	err = addCmd.Run()
	if err != nil {
		b.Fatal(err)
	}

	commitCmd := exec.CommandContext(
		b.Context(),
		gitBinaryName,
		gitCmdCommit,
		flagM,
		"initial commit",
	)

	commitCmd.Dir = cloneDir

	err = commitCmd.Run()
	if err != nil {
		b.Fatal(err)
	}

	pushCmd := exec.CommandContext(
		b.Context(),
		gitBinaryName,
		gitCmdPush,
		flagSetUpstream,
		consts.GitOrigin,
		testMainBranch,
	)

	pushCmd.Dir = cloneDir

	err = pushCmd.Run()
	if err != nil {
		b.Fatal(err)
	}

	return cloneDir
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
