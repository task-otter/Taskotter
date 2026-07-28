package app_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/store"
)

//nolint:gochecknoglobals // serializes stderr capture across parallel tests
var stderrCaptureMu sync.Mutex

func emptyRefInfo() store.RefInfo {
	return store.RefInfo{
		Repository:       "",
		RequestedVersion: "",
		SourceRef:        "",
		ResolvedCommit:   "",
		DefaultBranch:    "",
	}
}

func captureStderr(t *testing.T, runFn func()) string {
	t.Helper()

	stderrCaptureMu.Lock()
	defer stderrCaptureMu.Unlock()

	old := os.Stderr

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stderr = writer

	runFn()

	_ = writer.Close()
	os.Stderr = old

	var buf bytes.Buffer

	_, err = io.Copy(&buf, reader)
	if err != nil {
		t.Fatal(err)
	}

	_ = reader.Close()

	return buf.String()
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

	out := captureStderr(t, func() {
		app.ReportSyncRequired(result)
	})
	if !strings.Contains(out, "::error title=TaskOtter sync required::") {
		t.Fatalf("missing error annotation: %s", out)
	}

	if !strings.Contains(out, "TaskOtter opened sync PR #42: https://example.com/pull/42") {
		t.Fatalf("missing PR summary: %s", out)
	}

	if !strings.Contains(out, "::notice title=What happened::") {
		t.Fatalf("missing notice annotation: %s", out)
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

	out := captureStderr(t, func() {
		app.ReportSyncRequired(result)
	})
	if !strings.Contains(out, "did not return a pull request URL") {
		t.Fatalf("missing fallback summary: %s", out)
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
