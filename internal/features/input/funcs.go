// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package input

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/plan/domain"
	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/features/root/adapters/taskfile"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

// SyncInput maps resolved modules and dependencies into syncer input records.
func SyncInput(args *SyncInputArgs) (domain.SyncInput, error) {
	requestedSources := collectRequestedSources(args.Resolutions)
	allSources := append(append([]string{}, requestedSources...), args.DepSources...)

	input, err := assembleSyncInput(args, allSources)
	if err != nil {
		return domain.SyncInput{}, fmt.Errorf("assemble sync input: %w", err)
	}

	return input, nil
}

// Build assembles resolver results and dependencies into planner input.
func Build(args *SyncInputArgs) (Input, error) {
	args.TaskfileOps = defaultTaskfileOps()

	input, err := SyncInput(args)
	if err != nil {
		return Input{}, fmt.Errorf("sync input: %w", err)
	}

	return input, nil
}

func defaultTaskfileOps() taskfile.Ops {
	return taskfile.NewOps()
}

func assembleSyncInput(args *SyncInputArgs, allSources []string) (domain.SyncInput, error) {
	sourceToDest, err := resolvesvc.BuildDestinationMap(allSources)
	if err != nil {
		return domain.SyncInput{}, fmt.Errorf("build destination map: %w", err)
	}

	requestedRecords, destByTask := buildReqRecords(
		&buildReqArgs{cfg: args.Cfg, res: args.Resolutions, src: sourceToDest},
	)
	dependencyRecords := buildDepRecords(args.Cfg, args.DepSources, sourceToDest)

	return domain.SyncInput{
		Config:       args.Cfg,
		Snapshot:     args.Snapshot,
		TaskfileOps:  args.TaskfileOps,
		Requested:    requestedRecords,
		Dependencies: dependencyRecords,
		SourceToDest: sourceToDest,
		DestByTask:   destByTask,
	}, nil
}

func buildDepRecords(cfg *config.Config, deps []string, src map[string]string) []modRec {
	dependencyRecords := make([]modRec, consts.IndexZero, len(deps))

	for i := range deps {
		dep := deps[i]
		dest := src[dep]

		dependencyRecords = append(dependencyRecords, modRec{
			SourceModule:      dep,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(cfg.TargetFolder, dest),
		})
	}

	return dependencyRecords
}

func buildReqRecords(args *buildReqArgs) (
	records recMap,
	destinations map[string]string,
) {
	records = make(recMap)
	destinations = make(map[string]string)

	for i := range args.res {
		item := &args.res[i]
		dest := args.src[item.SourceModule]

		records[item.LogicalTask] = modRec{
			SourceModule:      item.SourceModule,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(args.cfg.TargetFolder, dest),
		}
		destinations[item.LogicalTask] = dest
	}

	return records, destinations
}

func collectRequestedSources(resolutions []resolvesvc.Resolution) []string {
	requestedSources := make([]string, consts.IndexZero, len(resolutions))

	for i := range resolutions {
		requestedSources = append(requestedSources, resolutions[i].SourceModule)
	}

	return requestedSources
}
