package normalizer_test

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/normalizer"
)

const destESLint = "eslint"

func TestNormalizeExamples(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"eslint/node/fnm/pnpm": destESLint,
		"eslint/node/nvm/pnpm": destESLint,
		"eslint/node/fnm/npm":  destESLint,
		"eslint/node/nvm/npm":  destESLint,
		"eslint/node/fnm/yarn": destESLint,
		"eslint/node/nvm/yarn": destESLint,
		"eslint/bun":           destESLint,
		"pnpm/fnm":             "pnpm",
		"npm/nvm":              "npm",
		"yarn/fnm":             "yarn",
		"corepack/fnm":         "corepack",
		"corepack/nvm":         "corepack",
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
		"corepack-fnm":         "corepack",
		"corepack-nvm":         "corepack",
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

	got, err := normalizer.Normalize("eslint/node/fnm/pnpm")
	if err != nil {
		t.Fatal(err)
	}

	if got != destESLint {
		t.Fatalf("got %q", got)
	}
}

func TestDestinationCollision(t *testing.T) {
	t.Parallel()

	_, err := normalizer.BuildDestinationMap([]string{"eslint/node/fnm/pnpm", "eslint/bun"})
	if err == nil {
		t.Fatal("expected collision error")
	}

	if !strings.Contains(err.Error(), "Destination collision") {
		t.Fatalf("unexpected error: %v", err)
	}
}
