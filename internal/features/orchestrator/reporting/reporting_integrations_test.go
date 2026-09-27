// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package reporting_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rundomain "github.com/task-otter/Taskotter/internal/features/orchestrator/domain"
	"github.com/task-otter/Taskotter/internal/features/orchestrator/reporting"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	writeOutputsArgs struct {
		Cfg        *config.Config
		Result     *rundomain.Result
		OutputPath string
	}
)

const (
	testSourceSHA      = "abc123"
	testTargetFolder   = "taskfiles"
	emptyJSONArray     = "[]"
	testPullRequestURL = "https://example.com/pull/42"

	testPRNumber42 = "42"
)

// TestReportSyncRequiredWithPullRequest verifies the behavior covered by this test.
func TestReportSyncRequiredWithPullRequest(t *testing.T) {
	t.Parallel()

	result := changedResult()

	result.PullRequestNumber = testPRNumber42
	result.PullRequestURL = testPullRequestURL

	var out bytes.Buffer

	reporting.ReportSyncRequiredTo(&out, result)

	got := out.String()

	assertContains(t, got, "::error title=TaskOtter sync required::")
	assertContains(t, got, "TaskOtter opened sync PR #42: "+testPullRequestURL)
	assertContains(t, got, "::notice title=What happened::")
}

// TestReportSyncRequiredWritesToStderr verifies the behavior covered by this test.
func TestReportSyncRequiredWritesToStderr(t *testing.T) {
	t.Parallel()

	reporting.ReportSyncRequired(changedResult())
}

// TestReportSyncRequiredWithUnknownPullRequestNumber verifies the behavior covered by this test.
func TestReportSyncRequiredWithUnknownPullRequestNumber(t *testing.T) {
	t.Parallel()

	result := changedResult()

	result.PullRequestURL = testPullRequestURL

	var out bytes.Buffer

	reporting.ReportSyncRequiredTo(&out, result)
	assertContains(t, out.String(), "sync PR #unknown")
}

// TestReportSyncRequiredWithoutPullRequest verifies the behavior covered by this test.
func TestReportSyncRequiredWithoutPullRequest(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer

	reporting.ReportSyncRequiredTo(&out, changedResult())
	assertContains(t, out.String(), "did not return a pull request URL")
}

// TestReportSyncUpToDateWritesNotice verifies the behavior covered by this test.
func TestReportSyncUpToDateWritesNotice(t *testing.T) {
	t.Parallel()

	result := emptyResult()

	result.SourceSHA = testSourceSHA
	reporting.ReportSyncUpToDate(result)
}

// TestResolvedTaskMarshalJSON verifies the behavior covered by this test.
func TestResolvedTaskMarshalJSON(t *testing.T) {
	t.Parallel()

	task := &rundomain.ResolvedTask{
		SourceModule:      "eslint/node/pnpm",
		DestinationModule: "eslint",
		Path:              "taskfiles/eslint",
	}

	data, err := task.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	assertContainsAll(t, string(data), []string{
		`"source_module":"eslint/node/pnpm"`,
		`"destination_module":"eslint"`,
		`"path":"taskfiles/eslint"`,
	})
}

// TestSyncRequired verifies the behavior covered by this test.
func TestSyncRequired(t *testing.T) {
	t.Parallel()

	if !reporting.SyncRequired(changedResult()) {
		t.Fatal("expected changed result to require sync")
	}

	if reporting.SyncRequired(emptyResult()) {
		t.Fatal("expected unchanged result not to require sync")
	}
}

// TestWriteActionOutputsToFile verifies the behavior covered by this test.
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
		"source-sha=" + testSourceSHA + "\n",
		"pull-request-number=42\n",
	})
}

// TestWriteActionOutputsToStdout verifies the behavior covered by this test.
func TestWriteActionOutputsToStdout(t *testing.T) {
	t.Parallel()

	err := reporting.WriteActionOutputs(emptyConfig(), newResultWithOutputs())
	if err != nil {
		t.Fatal(err)
	}
}

// TestWriteActionOutputsWrapsFileError verifies the behavior covered by this test.
func TestWriteActionOutputsWrapsFileError(t *testing.T) {
	t.Parallel()

	cfg := emptyConfig()

	cfg.GitHubOutput = filepath.Join(t.TempDir(), "missing", "output")

	err := reporting.WriteActionOutputs(cfg, emptyResult())
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

func changedResult() *rundomain.Result {
	result := emptyResult()

	result.Changed = true

	return result
}

func emptyConfig() *config.Config {
	return &config.Config{
		Tasks:              nil,
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
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

func emptyResult() *rundomain.Result {
	return &rundomain.Result{
		ResolvedOutput: rundomain.ResolvedOutput{
			ResolvedTasksJSON:    consts.Empty,
			ResolvedDependencies: consts.Empty,
		},
		PullRequestOutput: rundomain.PullRequestOutput{
			PullRequestNumber: consts.Empty,
			PullRequestURL:    consts.Empty,
		},
		Changed:      false,
		StoreVersion: consts.Empty,
		SourceRef:    consts.Empty,
		SourceSHA:    consts.Empty,
		TargetFolder: consts.Empty,
		Plan:         nil,
		Ref:          emptyRefInfo(),
	}
}

func newResultWithOutputs() *rundomain.Result {
	return &rundomain.Result{
		ResolvedOutput: rundomain.ResolvedOutput{
			ResolvedTasksJSON:    "{}",
			ResolvedDependencies: emptyJSONArray,
		},
		PullRequestOutput: rundomain.PullRequestOutput{
			PullRequestNumber: testPRNumber42,
			PullRequestURL:    testPullRequestURL,
		},
		Changed:      true,
		StoreVersion: "v1.2.3",
		SourceRef:    "refs/tags/v1.2.3",
		SourceSHA:    testSourceSHA,
		TargetFolder: testTargetFolder,
		Plan:         nil,
		Ref:          emptyRefInfo(),
	}
}

func writeOutputsAndRead(t *testing.T, args *writeOutputsArgs) string {
	t.Helper()

	err := reporting.WriteActionOutputs(args.Cfg, args.Result)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(args.OutputPath)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}
