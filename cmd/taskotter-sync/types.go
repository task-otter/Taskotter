// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"context"

	syncrun "github.com/task-otter/Taskotter/internal/features/syncrun/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	runSyncFn = func(context.Context, *config.Config) (*syncrun.Result, error)
	wireRunFn = func(context.Context, *config.Config) (runSyncFn, error)
)
