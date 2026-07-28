package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/store"
)

func emptyRefInfo() store.RefInfo {
	return store.RefInfo{
		Repository:       "",
		RequestedVersion: "",
		SourceRef:        "",
		ResolvedCommit:   "",
		DefaultBranch:    "",
	}
}

func TestReportSyncRequiredWithPullRequest(t *testing.T) {
	t.Parallel()

	result := &app.Result{
		Changed:              true,
		StoreVersion:         "",
		SourceRef:            "",
		SourceSHA:            "",
		TargetFolder:         "",
		ResolvedTasksJSON:    "",
		ResolvedDependencies: "",
		PullRequestNumber:    "42",
		PullRequestURL:       "https://example.com/pull/42",
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}

	var out bytes.Buffer

	app.ReportSyncRequiredTo(&out, result)

	if !strings.Contains(out.String(), "::error title=TaskOtter sync required::") {
		t.Fatalf("missing error annotation: %s", out.String())
	}

	if !strings.Contains(
		out.String(),
		"TaskOtter opened sync PR #42: https://example.com/pull/42",
	) {
		t.Fatalf("missing PR summary: %s", out.String())
	}

	if !strings.Contains(out.String(), "::notice title=What happened::") {
		t.Fatalf("missing notice annotation: %s", out.String())
	}
}

func TestReportSyncRequiredWithUnknownPullRequestNumber(t *testing.T) {
	t.Parallel()

	result := &app.Result{Changed: true, PullRequestURL: "https://example.com/pull/42"}

	var out bytes.Buffer
	app.ReportSyncRequiredTo(&out, result)

	if !strings.Contains(out.String(), "sync PR #unknown") {
		t.Fatalf("missing unknown PR number fallback: %s", out.String())
	}
}

func TestReportSyncRequiredWithoutPullRequest(t *testing.T) {
	t.Parallel()

	result := &app.Result{
		Changed:              true,
		StoreVersion:         "",
		SourceRef:            "",
		SourceSHA:            "",
		TargetFolder:         "",
		ResolvedTasksJSON:    "",
		ResolvedDependencies: "",
		PullRequestNumber:    "",
		PullRequestURL:       "",
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}

	var out bytes.Buffer

	app.ReportSyncRequiredTo(&out, result)

	if !strings.Contains(out.String(), "did not return a pull request URL") {
		t.Fatalf("missing fallback summary: %s", out.String())
	}
}

func TestSyncRequired(t *testing.T) {
	t.Parallel()

	changed := &app.Result{
		Changed:              true,
		StoreVersion:         "",
		SourceRef:            "",
		SourceSHA:            "",
		TargetFolder:         "",
		ResolvedTasksJSON:    "",
		ResolvedDependencies: "",
		PullRequestNumber:    "",
		PullRequestURL:       "",
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}
	if !app.SyncRequired(changed) {
		t.Fatal("expected changed result to require sync")
	}

	unchanged := &app.Result{
		Changed:              false,
		StoreVersion:         "",
		SourceRef:            "",
		SourceSHA:            "",
		TargetFolder:         "",
		ResolvedTasksJSON:    "",
		ResolvedDependencies: "",
		PullRequestNumber:    "",
		PullRequestURL:       "",
		Plan:                 nil,
		Ref:                  emptyRefInfo(),
	}
	if app.SyncRequired(unchanged) {
		t.Fatal("expected unchanged result not to require sync")
	}
}

func TestWriteActionOutputsToFile(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "github-output")
	cfg := &config.Config{GitHubOutput: outputPath}
	result := &app.Result{
		Changed:              true,
		StoreVersion:         "v1.2.3",
		SourceRef:            "refs/tags/v1.2.3",
		SourceSHA:            "abc123",
		TargetFolder:         "taskfiles",
		ResolvedTasksJSON:    "{}",
		ResolvedDependencies: "[]",
		PullRequestNumber:    "42",
		PullRequestURL:       "https://example.com/pull/42",
	}

	err := app.WriteActionOutputs(cfg, result)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"changed=true\n",
		"source-sha=abc123\n",
		"pull-request-number=42\n",
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("output missing %q: %s", want, data)
		}
	}
}

func TestWriteActionOutputsWrapsFileError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{GitHubOutput: filepath.Join(t.TempDir(), "missing", "output")}
	err := app.WriteActionOutputs(cfg, &app.Result{})
	if err == nil {
		t.Fatal("expected write output error")
	}
}
