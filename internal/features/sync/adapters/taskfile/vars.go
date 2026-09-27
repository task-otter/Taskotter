// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"errors"

	"github.com/task-otter/Taskotter/internal/features/sync/ports"
)

var (
	_ ports.RootTemplateProvider = Ops{}
	_ ports.IncludeRewriter      = Ops{}
	_ ports.RootTaskfileUpdater  = Ops{}

	errNoModuleVars = errors.New("module Taskfile has no vars")
)
