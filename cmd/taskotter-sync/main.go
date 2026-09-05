// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package main is the entry point for the taskotter-sync GitHub Action binary.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	syncrun "github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	runSyncFn = func(context.Context, *config.Config) (*syncrun.Result, error)

	wireRunFn = func(context.Context, *config.Config) (runSyncFn, error)
)

const (
	runTimeout     = 15 * time.Minute
	exitSuccess    = 0
	exitError      = 1
	errWireOrchFmt = "wire orchestrator: %w"
)

//nolint:gochecknoglobals // seams so tests can drive main without exiting the test process
var (
	exitFunc           = os.Exit
	stdout   io.Writer = os.Stdout
	stderr   io.Writer = os.Stderr
	wireRun  wireRunFn = defaultWireRun
)

func main() {
	exitFunc(run())
}

func run() int {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)

	defer cancel()

	cfg, result, err := loadRunAndWrite(ctx)
	if err != nil {
		reportError(err, "")

		return exitError
	}

	return reportResult(cfg, result)
}

func loadRunAndWrite(ctx context.Context) (cfg *config.Config, result *syncrun.Result, err error) {
	cfg, err = config.LoadFromEnv()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	result, err = runOrchestrator(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("run sync: %w", err)
	}

	err = writeOutputs(cfg, result)
	if err != nil {
		return nil, nil, fmt.Errorf("write action outputs: %w", err)
	}

	return cfg, result, nil
}

func defaultWireRun(ctx context.Context, cfg *config.Config) (runSyncFn, error) {
	orch, err := WireOrchestrator(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf(errWireOrchFmt, err)
	}

	return orch.Run, nil
}

func runOrchestrator(ctx context.Context, cfg *config.Config) (*syncrun.Result, error) {
	runSync, err := wireRun(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf(errWireOrchFmt, err)
	}

	result, err := runSync(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("run orchestrator: %w", err)
	}

	return result, nil
}

func writeOutputs(cfg *config.Config, result *syncrun.Result) error {
	err := syncrun.WriteActionOutputs(cfg, result)
	if err != nil {
		return fmt.Errorf("write outputs: %w", err)
	}

	return nil
}

func reportResult(cfg *config.Config, result *syncrun.Result) int {
	if result.Changed {
		return handleChanged(cfg, result)
	}

	return handleUnchanged(cfg, result)
}

func reportError(err error, prefix string) {
	iox.FprintfBestEffortf(stderr, "::error::%s%v\n", prefix, err)
}

func handleChanged(cfg *config.Config, result *syncrun.Result) int {
	writeErr := iox.Fprintln(stdout, "TaskOtter produced changes.")
	if writeErr != nil {
		return exitError
	}

	if cfg.FailOnChanges {
		syncrun.ReportSyncRequired(result)

		return exitError
	}

	return exitSuccess
}

func handleUnchanged(cfg *config.Config, result *syncrun.Result) int {
	writeErr := iox.Fprintln(stdout, "TaskOtter completed with no changes.")
	if writeErr != nil {
		return exitError
	}

	if cfg.FailOnChanges {
		syncrun.ReportSyncUpToDate(result)
	}

	return exitSuccess
}
