package normalizer_test

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/normalizer"
)

const (
	destCorepack = "corepack"
	destESLint   = "eslint"
	srcESLint    = "eslint/node/fnm/pnpm"
)

func TestNormalizeExamples(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		srcESLint:              destESLint,
		"eslint/node/nvm/pnpm": destESLint,
		"eslint/node/fnm/npm":  destESLint,
		"eslint/node/nvm/npm":  destESLint,
		"eslint/node/fnm/yarn": destESLint,
		"eslint/node/nvm/yarn": destESLint,
		"eslint/bun":           destESLint,
		"pnpm/fnm":             "pnpm",
		"npm/nvm":              "npm",
		"yarn/fnm":             "yarn",
		"corepack/fnm":         destCorepack,
		"corepack/nvm":         destCorepack,
		"eslint-pnpm-fnm":      destESLint,
		"eslint-pnpm-nvm":      destESLint,
		"eslint-npm-fnm":       destESLint,
		"eslint-npm-nvm":       destESLint,
		"eslint-yarn-fnm":      destESLint,
		"eslint-yarn-nvm":      destESLint,
		"eslint-bun":           destESLint,
		"pnpm-fnm":             "pnpm",
		"npm-nvm":              "npm",
		"yarn-fnm":             "yarn",
		"corepack-fnm":         destCorepack,
		"corepack-nvm":         destCorepack,
		"fnm":                  "fnm",
		"nvm":                  "nvm",
		"bun":                  "bun",
		"go":                   "go",
	}
	for source, want := range cases {
		got, err := normalizer.Normalize(source)
		if err != nil {
			t.Fatalf("Normalize(%q) error = %v", source, err)
		}

		if got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", source, got, want)
		}
	}
}

func TestLongestSuffixFirst(t *testing.T) {
	t.Parallel()

	got, err := normalizer.Normalize(srcESLint)
	if err != nil {
		t.Fatal(err)
	}

	if got != destESLint {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeRejectsEmptySource(t *testing.T) {
	t.Parallel()

	_, err := normalizer.Normalize("")
	if err == nil {
		t.Fatal("expected empty source error")
	}
}

func TestBuildDestinationMapPropagatesNormalizeError(t *testing.T) {
	t.Parallel()

	_, err := normalizer.BuildDestinationMap([]string{""})
	if err == nil {
		t.Fatal("expected normalize error")
	}
}

func TestDestinationCollision(t *testing.T) {
	t.Parallel()

	_, err := normalizer.BuildDestinationMap([]string{srcESLint, "eslint/bun"})
	if err == nil {
		t.Fatal("expected collision error")
	}

	if !strings.Contains(err.Error(), "Destination collision") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildDestinationMapSortsSources(t *testing.T) {
	t.Parallel()

	mapping, err := normalizer.BuildDestinationMap([]string{"go", srcESLint})
	if err != nil {
		t.Fatal(err)
	}

	got := normalizer.SortedSources(mapping)

	want := []string{srcESLint, "go"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedSources() = %#v, want %#v", got, want)
		}
	}
}
