// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	writeOutputsArgs struct {
		Cfg        *config.Config
		Result     *service.Result
		OutputPath string
	}
)

const (
	testPullRequestURL = "https://example.com/pull/42"

	testPRNumber42 = "42"
)

// TestReportSyncRequiredWithPullRequest verifies the report includes the opened pull request summary.
func TestReportSyncRequiredWithPullRequest(t *testing.T) {
	t.Parallel()

	result := changedResult()

	result.PullRequestNumber = testPRNumber42
	result.PullRequestURL = testPullRequestURL

	var out bytes.Buffer

	service.ReportSyncRequiredTo(&out, result)

	got := out.String()

	assertContains(t, got, "::error title=TaskOtter sync required::")
	assertContains(t, got, "TaskOtter opened sync PR #42: "+testPullRequestURL)
	assertContains(t, got, "::notice title=What happened::")
}

// TestReportSyncRequiredWithUnknownPullRequestNumber verifies an unknown PR number falls back gracefully.
func TestReportSyncRequiredWithUnknownPullRequestNumber(t *testing.T) {
	t.Parallel()

	result := changedResult()

	result.PullRequestURL = testPullRequestURL

	var out bytes.Buffer

	service.ReportSyncRequiredTo(&out, result)
	assertContains(t, out.String(), "sync PR #unknown")
}

// TestReportSyncRequiredWithoutPullRequest verifies the report falls back when no PR URL is set.
func TestReportSyncRequiredWithoutPullRequest(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	service.ReportSyncRequiredTo(&out, changedResult())
	assertContains(t, out.String(), "did not return a pull request URL")
}

// TestSyncRequired verifies changed results require sync and unchanged ones do not.
func TestSyncRequired(t *testing.T) {
	t.Parallel()

	if !service.SyncRequired(changedResult()) {
		t.Fatal("expected changed result to require sync")
	}

	if service.SyncRequired(emptyResult()) {
		t.Fatal("expected unchanged result not to require sync")
	}
}

// TestWriteActionOutputsToFile verifies action outputs are written to the GitHub output file.
func TestWriteActionOutputsToFile(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "github-output")
	cfg := emptyConfig()

	cfg.GitHubOutput = outputPath

	data := writeOutputsAndRead(t, &writeOutputsArgs{
		Cfg:        cfg,
		Result:     newResultWithOutputs(),
		OutputPath: outputPath,
	})

	assertContainsAll(t, data, []string{
		"changed=true\n",
		"source-sha=abc123\n",
		"pull-request-number=42\n",
	})
}

// TestWriteActionOutputsWrapsFileError verifies a missing output path returns a wrapped error.
func TestWriteActionOutputsWrapsFileError(t *testing.T) {
	t.Parallel()

	cfg := emptyConfig()

	cfg.GitHubOutput = filepath.Join(t.TempDir(), "missing", "output")

	err := service.WriteActionOutputs(cfg, emptyResult())
	if err == nil {
		t.Fatal("expected write output error")
	}
}

func assertContains(t *testing.T, haystack, want string) {
	t.Helper()

	if !strings.Contains(haystack, want) {
		t.Fatalf("missing %q: %s", want, haystack)
	}
}

func assertContainsAll(t *testing.T, haystack string, wants []string) {
	t.Helper()

	for i := range wants {
		if !strings.Contains(haystack, wants[i]) {
			t.Fatalf("output missing %q: %s", wants[i], haystack)
		}
	}
}

func changedResult() *service.Result {
	result := emptyResult()

	result.Changed = true

	return result
}

func emptyConfig() *config.Config {
	return &config.Config{
		Tasks:              nil,
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
		NodeVersionManager: consts.Empty,
		IncludesDoc:        false,
		SyncRoot:           false,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       consts.Empty,
		RootTaskfile:       consts.Empty,
		GitHubToken:        consts.Empty,
		Workspace:          consts.Empty,
		Repository:         consts.Empty,
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}
}

func emptyRefInfo() storedomain.RefInfo {
	return storedomain.RefInfo{
		Repository:       consts.Empty,
		RequestedVersion: consts.Empty,
		SourceRef:        consts.Empty,
		ResolvedCommit:   consts.Empty,
		DefaultBranch:    consts.Empty,
	}
}

func emptyResult() *service.Result {
	return &service.Result{
		Changed:              false,
		StoreVersion:         consts.Empty,
		SourceRef:            consts.Empty,
		SourceSHA:            consts.Empty,
		TargetFolder:         consts.Empty,
		ResolvedTasksJSON:    consts.Empty,
		ResolvedDependencies: consts.Empty,
		PullRequestNumber:    consts.Empty,
		PullRequestURL:       consts.Empty,
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}
}

func newResultWithOutputs() *service.Result {
	return &service.Result{
		Changed:              true,
		StoreVersion:         "v1.2.3",
		SourceRef:            "refs/tags/v1.2.3",
		SourceSHA:            "abc123",
		TargetFolder:         testTargetFolder,
		ResolvedTasksJSON:    "{}",
		ResolvedDependencies: "[]",
		PullRequestNumber:    testPRNumber42,
		PullRequestURL:       testPullRequestURL,
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}
}

func writeOutputsAndRead(t *testing.T, args *writeOutputsArgs) string {
	t.Helper()

	err := service.WriteActionOutputs(args.Cfg, args.Result)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(args.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}
