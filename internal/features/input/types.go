// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"github.com/task-otter/Taskotter/internal/features/plan/domain"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/root/ports"
	"github.com/task-otter/Taskotter/internal/features/state/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	// Input is the resolved module set consumed by the sync planner.
	Input = domain.SyncInput
	// Snapshot exposes store snapshot data needed during planning.
	Snapshot = ports.Snapshot
	// BuildInput contains resolver output and configuration for Build.
	BuildInput struct {
		Cfg         *config.Config
		Snapshot    Snapshot
		Resolutions []resolvesvc.Resolution
		DepSources  []string
	}
	// SyncInputArgs contains inputs used to build synchronization state.
	SyncInputArgs struct {
		Cfg         *config.Config
		Snapshot    ports.Snapshot
		TaskfileOps ports.TaskfileOps
		Resolutions []resolvesvc.Resolution
		DepSources  []string
	}
	buildReqArgs struct {
		cfg *config.Config
		src map[string]string
		res []resolvesvc.Resolution
	}
	modRec = lockmodel.ModuleRecord
	recMap = map[string]modRec
	// Resolution keeps the input API discoverable at this feature boundary.
	Resolution = resolvesvc.Resolution
	// Config aliases the shared validated configuration type.
	Config = config.Config
)
