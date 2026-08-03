// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/store"
)

const (
	testPullRequestURL = "https://example.com/pull/42"
	testPRNumber42     = "42"
)

func emptyRefInfo() store.RefInfo {
	return store.RefInfo{
		Repository:       consts.Empty,
		RequestedVersion: consts.Empty,
		SourceRef:        consts.Empty,
		ResolvedCommit:   consts.Empty,
		DefaultBranch:    consts.Empty,
	}
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

func emptyResult() *app.Result {
	return &app.Result{
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

// TestReportSyncRequiredWithPullRequest verifies the report includes the opened pull request summary.
func TestReportSyncRequiredWithPullRequest(t *testing.T) {
	t.Parallel()

	result := &app.Result{
		Changed:              true,
		StoreVersion:         consts.Empty,
		SourceRef:            consts.Empty,
		SourceSHA:            consts.Empty,
		TargetFolder:         consts.Empty,
		ResolvedTasksJSON:    consts.Empty,
		ResolvedDependencies: consts.Empty,
		PullRequestNumber:    testPRNumber42,
		PullRequestURL:       testPullRequestURL,
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}

	var out bytes.Buffer

	app.ReportSyncRequiredTo(&out, result)

	if !strings.Contains(out.String(), "::error title=TaskOtter sync required::") {
		t.Fatalf("missing error annotation: %s", out.String())
	}

	if !strings.Contains(out.String(), "TaskOtter opened sync PR #42: "+testPullRequestURL) {
		t.Fatalf("missing PR summary: %s", out.String())
	}

	if !strings.Contains(out.String(), "::notice title=What happened::") {
		t.Fatalf("missing notice annotation: %s", out.String())
	}
}

// TestReportSyncRequiredWithUnknownPullRequestNumber verifies an unknown PR number falls back gracefully.
func TestReportSyncRequiredWithUnknownPullRequestNumber(t *testing.T) {
	t.Parallel()

	result := emptyResult()

	result.Changed = true
	result.PullRequestURL = testPullRequestURL

	var out bytes.Buffer

	app.ReportSyncRequiredTo(&out, result)

	if !strings.Contains(out.String(), "sync PR #unknown") {
		t.Fatalf("missing unknown PR number fallback: %s", out.String())
	}
}

// TestReportSyncRequiredWithoutPullRequest verifies the report falls back when no PR URL is set.
func TestReportSyncRequiredWithoutPullRequest(t *testing.T) {
	t.Parallel()

	result := &app.Result{
		Changed:              true,
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

	var out bytes.Buffer

	app.ReportSyncRequiredTo(&out, result)

	if !strings.Contains(out.String(), "did not return a pull request URL") {
		t.Fatalf("missing fallback summary: %s", out.String())
	}
}

// TestSyncRequired verifies changed results require sync and unchanged ones do not.
func TestSyncRequired(t *testing.T) {
	t.Parallel()

	changed := &app.Result{
		Changed:              true,
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

	if !app.SyncRequired(changed) {
		t.Fatal("expected changed result to require sync")
	}

	unchanged := &app.Result{
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

	if app.SyncRequired(unchanged) {
		t.Fatal("expected unchanged result not to require sync")
	}
}

func newResultWithOutputs() *app.Result {
	return &app.Result{
		Changed:              true,
		StoreVersion:         "v1.2.3",
		SourceRef:            "refs/tags/v1.2.3",
		SourceSHA:            "abc123",
		TargetFolder:         "taskfiles",
		ResolvedTasksJSON:    "{}",
		ResolvedDependencies: "[]",
		PullRequestNumber:    testPRNumber42,
		PullRequestURL:       testPullRequestURL,
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}
}

func writeOutputsAndRead(
	t *testing.T,
	cfg *config.Config,
	result *app.Result,
	outputPath string,
) string {
	t.Helper()

	err := app.WriteActionOutputs(cfg, result)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func assertContainsAll(t *testing.T, haystack string, wants []string) {
	t.Helper()

	for i := range wants {
		if !strings.Contains(haystack, wants[i]) {
			t.Fatalf("output missing %q: %s", wants[i], haystack)
		}
	}
}

// TestWriteActionOutputsToFile verifies action outputs are written to the GitHub output file.
func TestWriteActionOutputsToFile(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "github-output")
	cfg := emptyConfig()

	cfg.GitHubOutput = outputPath

	data := writeOutputsAndRead(t, cfg, newResultWithOutputs(), outputPath)

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

	err := app.WriteActionOutputs(cfg, emptyResult())
	if err == nil {
		t.Fatal("expected write output error")
	}
}
