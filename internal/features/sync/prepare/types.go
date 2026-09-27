// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package prepare

import (
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	// PrepareSyncInputArgs bundles inputs for PrepareSyncInput.
	PrepareSyncInputArgs = struct {
		Cfg         *config.Config
		Snapshot    ports.Snapshot
		TaskfileOps ports.TaskfileOps
		Resolutions []resolvesvc.Resolution
		DepSources  []string
	}

	buildReqArgs = struct {
		cfg *config.Config
		src map[string]string
		res []resolvesvc.Resolution
	}

	modRec = lockmodel.ModuleRecord
	recMap = map[string]modRec
)
