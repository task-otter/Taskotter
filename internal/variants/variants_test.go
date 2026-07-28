package variants_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/variants"
)

func TestBuildSourceModule(t *testing.T) {
	t.Parallel()

	got, err := variants.BuildSourceModule("eslint", config.PMPnpm, config.VMFnm)
	if err != nil {
		t.Fatal(err)
	}

	if got != "eslint/node/fnm/pnpm" {
		t.Fatalf("got %q", got)
	}

	got, err = variants.BuildSourceModule("eslint", config.PMBun, "")
	if err != nil {
		t.Fatal(err)
	}

	if got != "eslint/bun" {
		t.Fatalf("got %q", got)
	}

	_, err = variants.BuildSourceModule("eslint", config.PMPnpm, "")
	if err == nil {
		t.Fatal("expected missing version manager error")
	}

	_, err = variants.BuildSourceModule("eslint", config.PackageManager("deno"), "")
	if err == nil {
		t.Fatal("expected invalid package manager error")
	}
}

func TestIsNodeToolVariant(t *testing.T) {
	t.Parallel()

	if !variants.IsNodeToolVariant("eslint/node/fnm/pnpm", "eslint") {
		t.Fatal("expected variant")
	}

	if variants.IsNodeToolVariant("go", "go") {
		t.Fatal("go is not a node variant")
	}

	if variants.IsNodeToolVariant("eslint", "eslint") {
		t.Fatal("module equal to logical task is not a variant")
	}

	if variants.IsNodeToolVariant("prettier/node/fnm/pnpm", "eslint") {
		t.Fatal("different logical task is not a variant")
	}

	if variants.IsNodeToolVariant("eslint/node/fnm/deno", "eslint") {
		t.Fatal("unknown node suffix is not a variant")
	}
}

func TestStripOneSuffix(t *testing.T) {
	t.Parallel()

	got, ok := variants.StripOneSuffix("eslint/node/fnm/pnpm")
	if !ok || got != "eslint" {
		t.Fatalf("got %q ok=%t", got, ok)
	}

	got, ok = variants.StripOneSuffix("eslint")
	if ok || got != "eslint" {
		t.Fatalf("got %q ok=%t", got, ok)
	}

	got, ok = variants.StripOneSuffix("eslint-fnm")
	if !ok || got != "eslint" {
		t.Fatalf("got %q ok=%t", got, ok)
	}

	got, ok = variants.StripOneSuffix("/bun")
	if ok || got != "/bun" {
		t.Fatalf("got %q ok=%t", got, ok)
	}
}
