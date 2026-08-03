// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package variants_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/variants"
)

const (
	taskESLint       = "eslint"
	srcESLintFnmPnpm = "eslint/node/fnm/pnpm"
	fmtGotQ          = "got %q"
	fmtGotStrippedQ  = "got %q stripped=%t"
	suffixBun        = "/bun"
)

func assertBuildSourceModule(
	t *testing.T,
	task string,
	pm config.PackageManager,
	vm config.VersionManager,
	want string,
) {
	t.Helper()

	got, err := variants.BuildSourceModule(task, pm, vm)
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf(fmtGotQ, got)
	}
}

func assertBuildSourceModuleError(
	t *testing.T,
	task string,
	pm config.PackageManager,
	vm config.VersionManager,
) {
	t.Helper()

	_, err := variants.BuildSourceModule(task, pm, vm)
	if err == nil {
		t.Fatal("expected BuildSourceModule error")
	}
}

// TestBuildSourceModule verifies source module names build correctly and reject invalid combinations.
func TestBuildSourceModule(t *testing.T) {
	t.Parallel()

	assertBuildSourceModule(t, taskESLint, config.PMPnpm, config.VMFnm, srcESLintFnmPnpm)
	assertBuildSourceModule(t, taskESLint, config.PMBun, consts.Empty, "eslint/bun")
	assertBuildSourceModuleError(t, taskESLint, config.PMPnpm, consts.Empty)
	assertBuildSourceModuleError(t, taskESLint, config.PackageManager("deno"), consts.Empty)
}

// TestIsNodeToolVariant verifies node tool variant module names are correctly identified.
func TestIsNodeToolVariant(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		moduleName  string
		logicalTask string
		want        bool
	}{
		{"variant", srcESLintFnmPnpm, taskESLint, true},
		{"non-node task", consts.Go, consts.Go, false},
		{"equal to logical task", taskESLint, taskESLint, false},
		{"different logical task", "prettier/node/fnm/pnpm", taskESLint, false},
		{"unknown node suffix", "eslint/node/fnm/deno", taskESLint, false},
	}

	for i := range cases {
		tc := &cases[i]
		got := variants.IsNodeToolVariant(tc.moduleName, tc.logicalTask)

		if got != tc.want {
			t.Fatalf(
				"IsNodeToolVariant(%q, %q) = %t, want %t",
				tc.moduleName,
				tc.logicalTask,
				got,
				tc.want,
			)
		}
	}
}

// TestStripOneSuffix verifies a single matching suffix is stripped from module names.
func TestStripOneSuffix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		input        string
		wantResult   string
		wantStripped bool
	}{
		{
			name: "variant with two segments", input: srcESLintFnmPnpm,
			wantStripped: true, wantResult: taskESLint,
		},
		{name: "no known suffix", input: taskESLint, wantStripped: false, wantResult: taskESLint},
		{name: "single suffix", input: "eslint-fnm", wantStripped: true, wantResult: taskESLint},
		{
			name:         "strip would empty name",
			input:        suffixBun,
			wantStripped: false,
			wantResult:   suffixBun,
		},
	}

	for i := range cases {
		tc := &cases[i]
		got, stripped := variants.StripOneSuffix(tc.input)

		if stripped != tc.wantStripped || got != tc.wantResult {
			t.Fatalf(fmtGotStrippedQ, got, stripped)
		}
	}
}
