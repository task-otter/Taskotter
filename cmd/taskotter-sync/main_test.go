// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	syncrun "github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
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

var (
	errStubRun      = errors.New("stub run failure")
	mainTestStateMu sync.Mutex
)

// TestMainExitsWithErrorWhenConfigMissing verifies main reports a failure exit code.
func TestMainExitsWithErrorWhenConfigMissing(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	code := exitError * consts.IndexZero

	swapExitFunc(t, func(got int) { code = got })
	captureStreams(t)
	clearActionEnv(t)

	main()

	if code != exitError {
		t.Fatalf(exitFmt, code, exitError)
	}
}

// TestRunReportsConfigFailure verifies a missing configuration exits with an error code.
func TestRunReportsConfigFailure(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	captureStreams(t)
	clearActionEnv(t)

	code := run()

	if code != exitError {
		t.Fatalf(exitFmt, code, exitError)
	}
}

// TestRunOrchestratorReportsWireFailure verifies orchestrator construction failures are wrapped.
func TestRunOrchestratorReportsWireFailure(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	result, err := runOrchestrator(t.Context(), invalidRepoOrchestratorConfig())
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected wire failure")
	}
}

// TestRunOrchestratorReportsRunFailure verifies orchestrator run failures are wrapped.
func TestRunOrchestratorReportsRunFailure(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	swapOrchestrator(t, &stubOrchestrator{result: nil, err: errStubRun})

	result, err := runOrchestrator(t.Context(), emptyConfig())
	iox.Discard(result)

	if !errors.Is(err, errStubRun) {
		t.Fatalf(errWantFmt, err, errStubRun)
	}
}

// TestLoadRunAndWriteReportsOutputFailure verifies an unwritable output path is reported.
func TestLoadRunAndWriteReportsOutputFailure(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	captureStreams(t)
	setValidActionEnv(t, filepath.Join(t.TempDir(), "missing", outputFileName))
	swapOrchestrator(t, &stubOrchestrator{result: unchangedResult(), err: nil})

	cfg, result, err := loadRunAndWrite(t.Context())
	iox.Discard2(cfg, result)

	if err == nil {
		t.Fatal("expected output write failure")
	}
}

// TestLoadRunAndWriteReportsRunFailure verifies orchestrator failures abort the run.
func TestLoadRunAndWriteReportsRunFailure(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	captureStreams(t)
	setValidActionEnv(t, filepath.Join(t.TempDir(), outputFileName))
	swapOrchestrator(t, &stubOrchestrator{result: nil, err: errStubRun})

	cfg, result, err := loadRunAndWrite(t.Context())
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
func TestRunSucceedsWithStubbedOrchestrator(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	captureStreams(t)
	setValidActionEnv(t, filepath.Join(t.TempDir(), outputFileName))
	swapOrchestrator(t, &stubOrchestrator{result: unchangedResult(), err: nil})

	code := run()

	if code != exitSuccess {
		t.Fatalf(exitFmt, code, exitSuccess)
	}
}

// TestReportResultChangedWithoutFailOnChanges verifies a changed run succeeds by default.
func TestReportResultChangedWithoutFailOnChanges(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	captureStreams(t)

	code := reportResult(emptyConfig(), changedResult())

	if code != exitSuccess {
		t.Fatalf(exitFmt, code, exitSuccess)
	}
}

// TestReportResultChangedWithFailOnChanges verifies fail-on-changes turns changes into failures.
func TestReportResultChangedWithFailOnChanges(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	captureStreams(t)

	code := reportResult(failOnChangesConfig(), changedResult())

	if code != exitError {
		t.Fatalf(exitFmt, code, exitError)
	}
}

// TestReportResultUnchangedWithFailOnChanges verifies an up-to-date run still succeeds.
func TestReportResultUnchangedWithFailOnChanges(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	captureStreams(t)

	code := reportResult(failOnChangesConfig(), unchangedResult())

	if code != exitSuccess {
		t.Fatalf(exitFmt, code, exitSuccess)
	}
}

// TestReportResultReportsWriteFailures verifies stdout failures become error exit codes.
func TestReportResultReportsWriteFailures(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)
	swapStdout(t, &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault})

	if reportResult(emptyConfig(), changedResult()) != exitError {
		t.Fatal("changed result should fail when stdout fails")
	}

	if reportResult(emptyConfig(), unchangedResult()) != exitError {
		t.Fatal("unchanged result should fail when stdout fails")
	}
}

// TestReportErrorWritesAnnotation verifies the error annotation reaches stderr.
func TestReportErrorWritesAnnotation(t *testing.T) {
	t.Parallel()
	lockMainTestState(t)

	var buf bytes.Buffer

	swapStderr(t, &buf)
	reportError(errStubRun, "prefix: ")

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

	mainTestStateMu.Lock()
	t.Cleanup(mainTestStateMu.Unlock)
}

func swapOrchestrator(t *testing.T, stub *stubOrchestrator) {
	t.Helper()

	original := wireRun

	wireRun = func(context.Context, *config.Config) (runSyncFn, error) {
		return stub.Run, nil
	}

	t.Cleanup(func() { wireRun = original })
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

	original, present := os.LookupEnv(key)

	err := os.Setenv(key, value)
	if err != nil {
		t.Fatalf("set %s: %v", key, err)
	}

	t.Cleanup(func() {
		if present {
			err := os.Setenv(key, original)
			if err != nil {
				t.Errorf("restore %s: %v", key, err)
			}

			return
		}

		err := os.Unsetenv(key)
		if err != nil {
			t.Errorf("unset %s: %v", key, err)
		}
	})
}

func swapExitFunc(t *testing.T, stub func(int)) {
	t.Helper()

	original := exitFunc

	exitFunc = stub

	t.Cleanup(func() { exitFunc = original })
}

func captureStreams(t *testing.T) {
	t.Helper()
	swapStdout(t, io.Discard)
	swapStderr(t, io.Discard)
}

func swapStderr(t *testing.T, writer io.Writer) {
	t.Helper()

	original := stderr

	stderr = writer

	t.Cleanup(func() { stderr = original })
}

func swapStdout(t *testing.T, writer io.Writer) {
	t.Helper()

	original := stdout

	stdout = writer

	t.Cleanup(func() { stdout = original })
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
