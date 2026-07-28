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

// PrepareSyncInput maps resolved modules and dependencies into syncer input records.
func PrepareSyncInput(
	cfg *config.Config,
	snapshot *store.Snapshot,
	resolutions []resolver.Resolution,
	depSources []string,
) (syncer.SyncInput, error) {
	requestedSources := make([]string, 0, len(resolutions))
	for _, res := range resolutions {
		requestedSources = append(requestedSources, res.SourceModule)
	}

	allSources := append(append([]string{}, requestedSources...), depSources...)

	sourceToDest, err := normalizer.BuildDestinationMap(allSources)
	if err != nil {
		return syncer.SyncInput{}, fmt.Errorf("build destination map: %w", err)
	}

	requestedRecords := make(map[string]syncer.ModuleRecord)
	destByTask := make(map[string]string)

	for _, res := range resolutions {
		dest := sourceToDest[res.SourceModule]
		requestedRecords[res.LogicalTask] = syncer.ModuleRecord{
			SourceModule:      res.SourceModule,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(cfg.TargetFolder, dest),
		}
		destByTask[res.LogicalTask] = dest
	}

	dependencyRecords := make([]syncer.ModuleRecord, 0, len(depSources))
	for _, dep := range depSources {
		dest := sourceToDest[dep]
		dependencyRecords = append(dependencyRecords, syncer.ModuleRecord{
			SourceModule:      dep,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(cfg.TargetFolder, dest),
		})
	}

	return syncer.SyncInput{
		Config:       cfg,
		Snapshot:     snapshot,
		Requested:    requestedRecords,
		Dependencies: dependencyRecords,
		SourceToDest: sourceToDest,
		DestByTask:   destByTask,
	}, nil
}
