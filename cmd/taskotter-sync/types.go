// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"context"

	rundomain "github.com/task-otter/Taskotter/internal/features/orchestrator/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	runSyncFn = func(context.Context, *config.Config) (*rundomain.Result, error)

	wireRunFn = func(context.Context, *config.Config) (runSyncFn, error)
)
