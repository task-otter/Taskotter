// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"github.com/task-otter/Taskotter/internal/features/plan/domain"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	// SyncInputArgs contains inputs used to build synchronization state.
	SyncInputArgs struct {
		Cfg         *config.Config
		Snapshot    Snapshot
		TaskfileOps domain.TaskfileOps
		Resolutions []resolvesvc.Resolution
		DepSources  []string
	}
	// Input is the resolved module set consumed by the sync planner.
	Input = domain.SyncInput
	// Snapshot exposes store snapshot data needed during planning.
	Snapshot     = domain.Snapshot
	buildReqArgs struct {
		cfg *config.Config
		src map[string]string
		res []resolvesvc.Resolution
	}
	modRec = lockmodel.ModuleRecord
	recMap = map[string]modRec
)
