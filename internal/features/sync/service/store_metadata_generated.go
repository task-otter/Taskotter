// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"slices"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	addCommonTasksInput struct {
		modulesByTask map[string][]string
		common        map[string]struct{}
		logicalTask   string
		exportedTasks []string
	}

	genRootTask = generatedRootTask
)

func addCommonExportedTasks(input *addCommonTasksInput) {
	for i := range input.exportedTasks {
		exported := input.exportedTasks[i]

		if _, ok := input.common[exported]; !ok {
			continue
		}

		input.modulesByTask[exported] = append(input.modulesByTask[exported], input.logicalTask)
	}
}

func buildGenRootTaskList(names []string, byTask map[string][]string) []genRootTask {
	generated := make([]genRootTask, consts.IndexZero, len(names))

	for i := range names {
		name := names[i]

		generated = append(generated, genRootTask{
			Name:    name,
			Modules: byTask[name],
		})
	}

	return generated
}

func buildGeneratedRootTasks(input *groupModulesInput) []genRootTask {
	input.common = commonStoreTaskNames(input.metadata)

	return finalizeGeneratedTasks(groupModulesByGeneratedTask(input))
}

func finalizeGeneratedTasks(modulesByTask map[string][]string) []genRootTask {
	names := qualifyingGeneratedTaskNames(modulesByTask)
	slices.Sort(names)

	return buildGenRootTaskList(names, modulesByTask)
}

func generatedTaskMetadata(input *groupModulesInput, logicalTask string) (storeTaskMetadata, bool) {
	record, foundRecord := input.requestedRecords[logicalTask]

	if !foundRecord {
		return storeTaskMetadata{
			Schema:        consts.Empty,
			Module:        consts.Empty,
			Taskfile:      consts.Empty,
			ExportedTasks: nil,
			Variants:      nil,
		}, false
	}

	return resolveStoreTaskMetadata(record.SourceModule, input.metadata)
}

func groupModulesByGeneratedTask(input *groupModulesInput) map[string][]string {
	modulesByTask := make(map[string][]string)

	for i := range input.requested {
		recordGeneratedTaskModules(modulesByTask, input, input.requested[i])
	}

	return modulesByTask
}

func qualifyingGeneratedTaskNames(modulesByTask map[string][]string) []string {
	names := make([]string, consts.IndexZero, len(modulesByTask))

	for name := range modulesByTask {
		if len(modulesByTask[name]) >= consts.IndexTwo {
			names = append(names, name)
		}
	}

	return names
}

func recordGeneratedTaskModules(byTask map[string][]string, input *groupModulesInput, task string) {
	meta, ok := generatedTaskMetadata(input, task)

	if !ok {
		return
	}

	addCommonExportedTasks(&addCommonTasksInput{
		modulesByTask: byTask,
		exportedTasks: meta.ExportedTasks,
		common:        input.common,
		logicalTask:   task,
	})
}
