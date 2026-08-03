// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package app

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/normalizer"
	"github.com/task-otter/Taskotter/internal/pathutil"
	"github.com/task-otter/Taskotter/internal/resolver"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

const zeroCapacity = 0

// PrepareSyncInput maps resolved modules and dependencies into syncer input records.
func PrepareSyncInput(
	cfg *config.Config,
	snapshot *store.Snapshot,
	resolutions []resolver.Resolution,
	depSources []string,
) (syncer.SyncInput, error) {
	requestedSources := collectRequestedSources(resolutions)
	allSources := append(append([]string{}, requestedSources...), depSources...)

	sourceToDest, err := normalizer.BuildDestinationMap(allSources)
	if err != nil {
		return syncer.SyncInput{}, fmt.Errorf("build destination map: %w", err)
	}

	requestedRecords, destByTask := buildRequestedRecords(cfg, resolutions, sourceToDest)
	dependencyRecords := buildDependencyRecords(cfg, depSources, sourceToDest)

	return syncer.SyncInput{
		Config:       cfg,
		Snapshot:     snapshot,
		Requested:    requestedRecords,
		Dependencies: dependencyRecords,
		SourceToDest: sourceToDest,
		DestByTask:   destByTask,
	}, nil
}

func collectRequestedSources(resolutions []resolver.Resolution) []string {
	requestedSources := make([]string, zeroCapacity, len(resolutions))

	for i := range resolutions {
		requestedSources = append(requestedSources, resolutions[i].SourceModule)
	}

	return requestedSources
}

func buildRequestedRecords(
	cfg *config.Config,
	resolutions []resolver.Resolution,
	sourceToDest map[string]string,
) (requestedRecords map[string]syncer.ModuleRecord, destByTask map[string]string) {
	requestedRecords = make(map[string]syncer.ModuleRecord)
	destByTask = make(map[string]string)

	for i := range resolutions {
		res := &resolutions[i]
		dest := sourceToDest[res.SourceModule]

		requestedRecords[res.LogicalTask] = syncer.ModuleRecord{
			SourceModule:      res.SourceModule,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(cfg.TargetFolder, dest),
		}
		destByTask[res.LogicalTask] = dest
	}

	return requestedRecords, destByTask
}

func buildDependencyRecords(
	cfg *config.Config,
	depSources []string,
	sourceToDest map[string]string,
) []syncer.ModuleRecord {
	dependencyRecords := make([]syncer.ModuleRecord, zeroCapacity, len(depSources))

	for i := range depSources {
		dep := depSources[i]
		dest := sourceToDest[dep]

		dependencyRecords = append(dependencyRecords, syncer.ModuleRecord{
			SourceModule:      dep,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(cfg.TargetFolder, dest),
		})
	}

	return dependencyRecords
}
