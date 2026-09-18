// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	syncrun "github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/testsupport"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
)

type (
	stubOrchestrator struct {
		result *syncrun.Result
		err    error
	}
)

const (
	exitFmt                 = "exit code = %d, want %d"
	sourceSHAHex            = "0123456789abcdef"
	errWantFmt              = "err = %v, want %v"
	outputFileName          = "out.txt"
	testRepository          = "owner/repo"
	errExpectedOrchestrator = "expected orchestrator"
)

var errStubRun = errors.New("stub run failure")

// TestMainExitsWithErrorWhenConfigMissing verifies main reports a failure exit code.
//
//nolint:paralleltest // mutates process-wide environment variables and shared main test state.
func TestMainExitsWithErrorWhenConfigMissing(t *testing.T) {
	lockMainTestState(t)

	application := newTestApp(t)
	code := exitError * consts.IndexZero

	application.exit = func(got int) { code = got }

	clearActionEnv(t)

	application.main()

	if code != exitError {
		t.Fatalf(exitFmt, code, exitError)
	}
}

// TestRunReportsConfigFailure verifies a missing configuration exits with an error code.
//
//nolint:paralleltest // mutates process-wide environment variables and shared main test state.
func TestRunReportsConfigFailure(t *testing.T) {
	lockMainTestState(t)
	clearActionEnv(t)

	code := newTestApp(t).run()

	if code != exitError {
		t.Fatalf(exitFmt, code, exitError)
	}
}

// TestRunOrchestratorReportsWireFailure verifies orchestrator construction failures are wrapped.
func TestRunOrchestratorReportsWireFailure(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	result, err := newTestApp(t).runOrchestrator(t.Context(), invalidRepoOrchestratorConfig())
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected wire failure")
	}
}

// TestRunOrchestratorReportsRunFailure verifies orchestrator run failures are wrapped.
func TestRunOrchestratorReportsRunFailure(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	application := newTestApp(t)
	swapOrchestrator(t, application, &stubOrchestrator{result: nil, err: errStubRun})

	result, err := application.runOrchestrator(t.Context(), emptyConfig())
	iox.Discard(result)

	if !errors.Is(err, errStubRun) {
		t.Fatalf(errWantFmt, err, errStubRun)
	}
}

// TestLoadRunAndWriteReportsOutputFailure verifies an unwritable output path is reported.
//
//nolint:paralleltest // mutates process-wide environment variables and shared main test state.
func TestLoadRunAndWriteReportsOutputFailure(t *testing.T) {
	lockMainTestState(t)
	setValidActionEnv(t, filepath.Join(t.TempDir(), "missing", outputFileName))

	application := newTestApp(t)
	swapOrchestrator(t, application, &stubOrchestrator{result: unchangedResult(), err: nil})

	cfg, result, err := application.loadRunAndWrite(t.Context())
	iox.Discard2(cfg, result)

	if err == nil {
		t.Fatal("expected output write failure")
	}
}

// TestLoadRunAndWriteReportsRunFailure verifies orchestrator failures abort the run.
//
//nolint:paralleltest // mutates process-wide environment variables and shared main test state.
func TestLoadRunAndWriteReportsRunFailure(t *testing.T) {
	lockMainTestState(t)
	setValidActionEnv(t, filepath.Join(t.TempDir(), outputFileName))

	application := newTestApp(t)
	swapOrchestrator(t, application, &stubOrchestrator{result: nil, err: errStubRun})

	cfg, result, err := application.loadRunAndWrite(t.Context())
	iox.Discard2(cfg, result)

	if !errors.Is(err, errStubRun) {
		t.Fatalf(errWantFmt, err, errStubRun)
	}
}

// TestDefaultWireRunBuildsRunFunc verifies the production seam wires adapters.
func TestDefaultWireRunBuildsRunFunc(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	runSync, err := defaultWireRun(t.Context(), emptyConfig())
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if runSync == nil {
		t.Fatal("expected a run function")
	}
}

// TestRunSucceedsWithStubbedOrchestrator verifies a clean run exits successfully.
//
//nolint:paralleltest // mutates process-wide environment variables and shared main test state.
func TestRunSucceedsWithStubbedOrchestrator(t *testing.T) {
	lockMainTestState(t)
	setValidActionEnv(t, filepath.Join(t.TempDir(), outputFileName))

	application := newTestApp(t)
	swapOrchestrator(t, application, &stubOrchestrator{result: unchangedResult(), err: nil})

	code := application.run()

	if code != exitSuccess {
		t.Fatalf(exitFmt, code, exitSuccess)
	}
}

// TestReportResultChangedWithoutFailOnChanges verifies a changed run succeeds by default.
func TestReportResultChangedWithoutFailOnChanges(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	application := newTestApp(t)

	code := application.reportResult(emptyConfig(), changedResult())

	if code != exitSuccess {
		t.Fatalf(exitFmt, code, exitSuccess)
	}
}

// TestReportResultChangedWithFailOnChanges verifies fail-on-changes turns changes into failures.
func TestReportResultChangedWithFailOnChanges(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	application := newTestApp(t)

	code := application.reportResult(failOnChangesConfig(), changedResult())

	if code != exitError {
		t.Fatalf(exitFmt, code, exitError)
	}
}

// TestReportResultUnchangedWithFailOnChanges verifies an up-to-date run still succeeds.
func TestReportResultUnchangedWithFailOnChanges(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	application := newTestApp(t)

	code := application.reportResult(failOnChangesConfig(), unchangedResult())

	if code != exitSuccess {
		t.Fatalf(exitFmt, code, exitSuccess)
	}
}

// TestReportResultReportsWriteFailures verifies stdout failures become error exit codes.
func TestReportResultReportsWriteFailures(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	application := newTestApp(t)

	application.stdout = &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault}

	if application.reportResult(emptyConfig(), changedResult()) != exitError {
		t.Fatal("changed result should fail when stdout fails")
	}

	if application.reportResult(emptyConfig(), unchangedResult()) != exitError {
		t.Fatal("unchanged result should fail when stdout fails")
	}
}

// TestReportErrorWritesAnnotation verifies the error annotation reaches stderr.
func TestReportErrorWritesAnnotation(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	var buf bytes.Buffer

	application := newTestApp(t)

	application.stderr = &buf
	application.reportError(errStubRun, "prefix: ")

	if !bytes.Contains(buf.Bytes(), []byte("::error::prefix: ")) {
		t.Fatalf("stderr = %q", buf.String())
	}
}

func (stub *stubOrchestrator) Run(
	ctx context.Context,
	cfg *config.Config,
) (*syncrun.Result, error) {
	iox.Discard2(ctx, cfg)

	return stub.result, stub.err
}

func lockMainTestState(t *testing.T) {
	t.Helper()
	t.Cleanup(testsupport.Lock())
}

func newTestApp(t *testing.T) *app {
	t.Helper()

	application := newApp()

	application.stdout = io.Discard
	application.stderr = io.Discard

	return application
}

func swapOrchestrator(t *testing.T, application *app, stub *stubOrchestrator) {
	t.Helper()

	original := application.wireRun

	application.wireRun = func(context.Context, *config.Config) (runSyncFn, error) {
		return stub.Run, nil
	}

	t.Cleanup(func() { application.wireRun = original })
}

func changedResult() *syncrun.Result {
	result := unchangedResult()

	result.Changed = true

	return result
}

func clearActionEnv(t *testing.T) {
	t.Helper()
	setActionEnv(t, consts.EnvGithubWorkspace, consts.Empty)
	setActionEnv(t, consts.InputGithubToken, consts.Empty)
	setActionEnv(t, "GITHUB_TOKEN", consts.Empty)
}

func emptyConfig() *config.Config {
	cfg := invalidRepoOrchestratorConfig()

	cfg.Repository = consts.Empty

	return cfg
}

func failOnChangesConfig() *config.Config {
	cfg := emptyConfig()

	cfg.FailOnChanges = true

	return cfg
}

func setValidActionEnv(t *testing.T, outputPath string) {
	t.Helper()
	setActionEnv(t, consts.EnvGithubWorkspace, t.TempDir())
	setActionEnv(t, consts.InputGithubToken, "token")
	setActionEnv(t, consts.InputTasks, consts.Go)
	setActionEnv(t, "GITHUB_REPOSITORY", testRepository)
	setActionEnv(t, "GITHUB_OUTPUT", outputPath)
}

func setActionEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func unchangedResult() *syncrun.Result {
	return &syncrun.Result{
		Changed:   false,
		SourceSHA: sourceSHAHex,
	}
}

// TestWireOrchestratorInvalidRepository verifies an invalid repository coordinate fails construction.
func TestWireOrchestratorInvalidRepository(t *testing.T) {
	t.Parallel()

	orch, err := WireOrchestrator(t.Context(), invalidRepoOrchestratorConfig())
	iox.Discard(orch)

	if err == nil {
		t.Fatal("expected repository parse error")
	}
}

// TestWireOrchestratorWithoutRepository verifies an empty repository wires successfully.
func TestWireOrchestratorWithoutRepository(t *testing.T) {
	t.Parallel()

	cfg := invalidRepoOrchestratorConfig()

	cfg.Repository = consts.Empty

	orch, err := WireOrchestrator(t.Context(), cfg)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if orch == nil {
		t.Fatal(errExpectedOrchestrator)
	}
}

// TestWireOrchestratorWithRepository verifies a valid repository wires successfully.
func TestWireOrchestratorWithRepository(t *testing.T) {
	t.Parallel()

	cfg := invalidRepoOrchestratorConfig()

	cfg.Repository = testRepository

	orch, err := WireOrchestrator(t.Context(), cfg)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if orch == nil {
		t.Fatal(errExpectedOrchestrator)
	}
}

func invalidRepoOrchestratorConfig() *config.Config {
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
		Repository:         "not-a-valid-repo",
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}
}
