// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"context"
	"io"
	"os"

	syncrun "github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	runSyncFn = func(context.Context, *config.Config) (*syncrun.Result, error)
	wireRunFn = func(context.Context, *config.Config) (runSyncFn, error)

	app struct {
		exit    func(int)
		stdout  io.Writer
		stderr  io.Writer
		wireRun wireRunFn
	}
)

func newApp() *app {
	return &app{
		exit:    os.Exit,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		wireRun: defaultWireRun,
	}
}
