// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	benchModuleCount   = 20
	benchIncludeCount  = 12
	benchModulePrefix  = "mod"
	benchGeneratedTask = "lint"
	benchIncludeFmt    = "  %s:\n    taskfile: ../../../%s/Taskfile.yml\n"
	benchTaskfileHead  = "version: \"3\"\nvars:\n  TOOL_VERSION: \"1.0.0\"\nincludes:\n"
	benchTaskfileTail  = "tasks:\n  lint:\n    cmds:\n      - echo lint\n"
	benchVarsTaskfile  = "version: \"3\"\nvars:\n  MOD_VERSION: \"\"\n  SHARED_FLAG: "
)

func benchModuleName(idx int) string {
	return benchModulePrefix + strconv.Itoa(idx)
}

func benchModuleNames(count int) []string {
	names := make([]string, consts.IndexZero, count)

	for idx := range count {
		names = append(names, benchModuleName(idx))
	}

	return names
}

// benchModuleTaskfile renders a module Taskfile with sibling includes and vars.
func benchModuleTaskfile() []byte {
	var out strings.Builder

	iox.Discard(iox.WriteStringFull(&out, benchTaskfileHead))

	for idx := range benchIncludeCount {
		name := benchModuleName(idx)
		iox.Discard(iox.Fprintf(&out, benchIncludeFmt, name, name))
	}

	iox.Discard(iox.WriteStringFull(&out, benchTaskfileTail))

	return []byte(out.String())
}

func benchNameMap(count int) map[string]string {
	mapping := make(map[string]string, count)

	for idx := range count {
		mapping[benchModuleName(idx)] = benchModuleName(idx)
	}

	return mapping
}

func benchModuleVarsTaskfile(name string) []byte {
	return []byte(benchVarsTaskfile + strconv.Quote(name) + consts.Newline + benchTaskfileTail)
}

func benchModuleTaskfiles() map[string][]byte {
	files := make(map[string][]byte, benchModuleCount)

	for idx := range benchModuleCount {
		name := benchModuleName(idx)

		files[name] = benchModuleVarsTaskfile(name)
	}

	return files
}

func benchRootUpdateInput() *rootupd.RootUpdateInput {
	names := benchModuleNames(benchModuleCount)

	return &rootupd.RootUpdateInput{
		Tasks:            names,
		TargetFolder:     targetFolderTaskfiles,
		RootTaskfileDir:  consts.Empty,
		DestByTask:       benchNameMap(benchModuleCount),
		ManagedTasks:     names,
		ModuleTaskfiles:  benchModuleTaskfiles(),
		GeneratedTasks:   []rootupd.GeneratedRootTask{{Name: benchGeneratedTask, Modules: names}},
		ManagedRootTasks: []string{benchGeneratedTask},
	}
}

func benchExistingRoot(b *testing.B) []byte {
	b.Helper()

	out, err := taskfile.UpdateRootTaskfile(taskfile.NewRootTemplate(), benchRootUpdateInput())
	if err != nil {
		b.Fatal(err)
	}

	return out
}

// BenchmarkRewriteIncludes measures rewriting module include paths in a Taskfile.
func BenchmarkRewriteIncludes(b *testing.B) {
	content := benchModuleTaskfile()
	mapping := benchNameMap(benchIncludeCount)
	fromDest := benchModuleName(consts.IndexZero)

	for b.Loop() {
		out, err := taskfile.RewriteIncludes(content, mapping, fromDest)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(out)
	}
}

// BenchmarkUpdateRootTaskfileFromTemplate measures generating a root Taskfile from scratch.
func BenchmarkUpdateRootTaskfileFromTemplate(b *testing.B) {
	input := benchRootUpdateInput()

	for b.Loop() {
		out, err := taskfile.UpdateRootTaskfile(taskfile.NewRootTemplate(), input)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(out)
	}
}

// BenchmarkUpdateRootTaskfileExisting measures merging managed includes into an existing root.
func BenchmarkUpdateRootTaskfileExisting(b *testing.B) {
	input := benchRootUpdateInput()
	existing := benchExistingRoot(b)

	for b.Loop() {
		out, err := taskfile.UpdateRootTaskfile(existing, input)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(out)
	}
}
