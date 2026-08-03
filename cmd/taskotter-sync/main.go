// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package main is the entry point for the taskotter-sync GitHub Action binary.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
)

const (
	runTimeout  = 15 * time.Minute
	exitSuccess = 0
	exitError   = 1
)

func main() {
	os.Exit(run())
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

func loadRunAndWrite(ctx context.Context) (*config.Config, *app.Result, error) {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	result, err := app.Run(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("run app: %w", err)
	}

	err = app.WriteActionOutputs(cfg, result)
	if err != nil {
		return nil, nil, fmt.Errorf("write outputs: %w", err)
	}

	return cfg, result, nil
}

func reportResult(cfg *config.Config, result *app.Result) int {
	if result.Changed {
		return handleChanged(cfg, result)
	}

	return handleUnchanged(cfg, result)
}

func reportError(err error, prefix string) {
	n, writeErr := fmt.Fprintf(os.Stderr, "::error::%s%v\n", prefix, err)

	if writeErr != nil || n < exitSuccess {
		return
	}
}

func handleChanged(cfg *config.Config, result *app.Result) int {
	n, writeErr := fmt.Fprintln(os.Stdout, "TaskOtter produced changes.")

	if writeErr != nil || n < exitSuccess {
		return exitError
	}

	if cfg.FailOnChanges {
		app.ReportSyncRequired(result)

		return exitError
	}

	return exitSuccess
}

func handleUnchanged(cfg *config.Config, result *app.Result) int {
	n, writeErr := fmt.Fprintln(os.Stdout, "TaskOtter completed with no changes.")

	if writeErr != nil || n < exitSuccess {
		return exitError
	}

	if cfg.FailOnChanges {
		app.ReportSyncUpToDate(result)
	}

	return exitSuccess
}
