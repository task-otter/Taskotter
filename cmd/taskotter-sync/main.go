// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/task-otter/Taskotter/internal/app"
	"github.com/task-otter/Taskotter/internal/config"
)

const runTimeout = 15 * time.Minute

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)

	defer cancel()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::%v\n", err)

		return 1
	}

	result, err := app.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::%v\n", err)

		return 1
	}

	err = app.WriteActionOutputs(cfg, result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::write outputs: %v\n", err)

		return 1
	}

	if result.Changed {
		_, _ = fmt.Fprintf(os.Stdout, "TaskOtter produced changes.\n")

		if cfg.FailOnChanges {
			app.ReportSyncRequired(result)

			return 1
		}

		return 0
	}

	_, _ = fmt.Fprintf(os.Stdout, "TaskOtter completed with no changes.\n")

	if cfg.FailOnChanges {
		app.ReportSyncUpToDate(result)
	}

	return 0
}
