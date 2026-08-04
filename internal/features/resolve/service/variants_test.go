// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	buildSourceModuleCase struct {
		task   string
		pkgMgr config.PackageManager
		vm     config.VersionManager
		want   string
	}

	nodeToolVariantCase struct {
		moduleName  string
		logicalTask string
		expected    bool
	}

	stripOneSuffixCase struct {
		input        string
		wantResult   string
		wantStripped bool
	}
)

const (
	fmtGotStrippedQ = "got %q stripped=%t"
	suffixBun       = "/bun"
)

func assertBuildSourceModule(t *testing.T, testCase *buildSourceModuleCase) {
	t.Helper()

	got, err := service.BuildSourceModule(testCase.task, testCase.pkgMgr, testCase.vm)
	if err != nil {
		t.Fatal(err)
	}

	if got != testCase.want {
		t.Fatalf(fmtGotQ, got)
	}
}

func assertBuildSourceModuleError(t *testing.T, testCase *buildSourceModuleCase) {
	t.Helper()

	got, err := service.BuildSourceModule(
		testCase.task,
		testCase.pkgMgr,
		testCase.vm,
	)
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected BuildSourceModule error")
	}
}

// TestBuildSourceModule verifies source module names build correctly and reject invalid combinations.
func TestBuildSourceModule(t *testing.T) {
	t.Parallel()

	assertBuildSourceModule(t, &buildSourceModuleCase{
		task: taskESLint, pkgMgr: config.PMPnpm, vm: config.VMFnm, want: srcESLintFnmPnpm,
	})
	assertBuildSourceModule(t, &buildSourceModuleCase{
		task:   taskESLint,
		pkgMgr: config.PackageManager(config.JSRuntimeBun),
		vm:     consts.Empty,
		want:   "eslint/bun",
	})
	assertBuildSourceModuleError(t, &buildSourceModuleCase{
		task: taskESLint, pkgMgr: config.PMPnpm, vm: consts.Empty, want: consts.Empty,
	})
	assertBuildSourceModuleError(t, &buildSourceModuleCase{
		task:   taskESLint,
		pkgMgr: config.PackageManager(pkgDeno),
		vm:     consts.Empty,
		want:   consts.Empty,
	})
}

func nodeToolVariantCases() []nodeToolVariantCase {
	return []nodeToolVariantCase{
		{moduleName: srcESLintFnmPnpm, logicalTask: taskESLint, expected: true},
		{moduleName: consts.Go, logicalTask: consts.Go, expected: false},
		{moduleName: taskESLint, logicalTask: taskESLint, expected: false},
		{moduleName: "prettier/node/fnm/pnpm", logicalTask: taskESLint, expected: false},
		{moduleName: "eslint/node/fnm/deno", logicalTask: taskESLint, expected: false},
	}
}

func assertNodeToolVariant(t *testing.T, testCase *nodeToolVariantCase) {
	t.Helper()

	got := service.IsNodeToolVariant(testCase.moduleName, testCase.logicalTask)

	if got != testCase.expected {
		t.Fatalf(
			"IsNodeToolVariant(%q, %q) = %t, want %t",
			testCase.moduleName,
			testCase.logicalTask,
			got,
			testCase.expected,
		)
	}
}

// TestIsNodeToolVariant verifies node tool variant module names are correctly identified.
func TestIsNodeToolVariant(t *testing.T) {
	t.Parallel()

	cases := nodeToolVariantCases()

	for i := range cases {
		assertNodeToolVariant(t, &cases[i])
	}
}

func stripOneSuffixPrimaryCases() []stripOneSuffixCase {
	return []stripOneSuffixCase{
		{input: srcESLintFnmPnpm, wantStripped: true, wantResult: taskESLint},
		{input: taskESLint, wantStripped: false, wantResult: taskESLint},
		{input: "eslint-fnm", wantStripped: true, wantResult: taskESLint},
	}
}

func stripOneSuffixEdgeCases() []stripOneSuffixCase {
	return []stripOneSuffixCase{
		{input: suffixBun, wantStripped: false, wantResult: suffixBun},
	}
}

func assertStripOneSuffix(t *testing.T, testCase *stripOneSuffixCase) {
	t.Helper()

	got, stripped := service.StripOneSuffix(testCase.input)

	if stripped != testCase.wantStripped || got != testCase.wantResult {
		t.Fatalf(fmtGotStrippedQ, got, stripped)
	}
}

func assertStripOneSuffixCases(t *testing.T, cases []stripOneSuffixCase) {
	t.Helper()

	for i := range cases {
		assertStripOneSuffix(t, &cases[i])
	}
}

// TestStripOneSuffix verifies a single matching suffix is stripped from module names.
func TestStripOneSuffix(t *testing.T) {
	t.Parallel()

	assertStripOneSuffixCases(t, stripOneSuffixPrimaryCases())
	assertStripOneSuffixCases(t, stripOneSuffixEdgeCases())
}
