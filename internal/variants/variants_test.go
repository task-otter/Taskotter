// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package variants_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/variants"
)

const taskESLint = "eslint"

func TestBuildSourceModule(t *testing.T) {
	t.Parallel()

	got, err := variants.BuildSourceModule(taskESLint, config.PMPnpm, config.VMFnm)
	if err != nil {
		t.Fatal(err)
	}

	if got != "eslint/node/fnm/pnpm" {
		t.Fatalf("got %q", got)
	}

	got, err = variants.BuildSourceModule(taskESLint, config.PMBun, "")
	if err != nil {
		t.Fatal(err)
	}

	if got != "eslint/bun" {
		t.Fatalf("got %q", got)
	}

	_, err = variants.BuildSourceModule(taskESLint, config.PMPnpm, "")
	if err == nil {
		t.Fatal("expected missing version manager error")
	}

	_, err = variants.BuildSourceModule(taskESLint, config.PackageManager("deno"), "")
	if err == nil {
		t.Fatal("expected invalid package manager error")
	}
}

func TestIsNodeToolVariant(t *testing.T) {
	t.Parallel()

	if !variants.IsNodeToolVariant("eslint/node/fnm/pnpm", taskESLint) {
		t.Fatal("expected variant")
	}

	if variants.IsNodeToolVariant("go", "go") {
		t.Fatal("go is not a node variant")
	}

	if variants.IsNodeToolVariant(taskESLint, taskESLint) {
		t.Fatal("module equal to logical task is not a variant")
	}

	if variants.IsNodeToolVariant("prettier/node/fnm/pnpm", taskESLint) {
		t.Fatal("different logical task is not a variant")
	}

	if variants.IsNodeToolVariant("eslint/node/fnm/deno", taskESLint) {
		t.Fatal("unknown node suffix is not a variant")
	}
}

func TestStripOneSuffix(t *testing.T) {
	t.Parallel()

	got, stripped := variants.StripOneSuffix("eslint/node/fnm/pnpm")

	if !stripped || got != taskESLint {
		t.Fatalf("got %q stripped=%t", got, stripped)
	}

	got, stripped = variants.StripOneSuffix(taskESLint)

	if stripped || got != taskESLint {
		t.Fatalf("got %q stripped=%t", got, stripped)
	}

	got, stripped = variants.StripOneSuffix("eslint-fnm")

	if !stripped || got != taskESLint {
		t.Fatalf("got %q stripped=%t", got, stripped)
	}

	got, stripped = variants.StripOneSuffix("/bun")

	if stripped || got != "/bun" {
		t.Fatalf("got %q stripped=%t", got, stripped)
	}
}
