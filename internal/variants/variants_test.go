package variants_test

import (
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/variants"
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
}

func TestIsNodeToolVariant(t *testing.T) {
	t.Parallel()

	if !variants.IsNodeToolVariant("eslint/node/fnm/pnpm", "eslint") {
		t.Fatal("expected variant")
	}

	if variants.IsNodeToolVariant("go", "go") {
		t.Fatal("go is not a node variant")
	}
}

func TestStripOneSuffix(t *testing.T) {
	t.Parallel()

	got, ok := variants.StripOneSuffix("eslint/node/fnm/pnpm")
	if !ok || got != "eslint" {
		t.Fatalf("got %q ok=%t", got, ok)
	}
}
