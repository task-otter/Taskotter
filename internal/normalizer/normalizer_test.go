// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package normalizer_test

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/normalizer"
)

const (
	destCorepack = "corepack"
	destESLint   = "eslint"
	srcESLint    = "eslint/node/fnm/pnpm"
	srcESLintBun = "eslint/bun"
	destPnpm     = "pnpm"
	destNpm      = "npm"
	destYarn     = "yarn"
	destFnm      = "fnm"
	destNvm      = "nvm"
	destBun      = "bun"
)

// TestNormalizeExamples verifies variant module source names normalize to expected destinations.
func TestNormalizeExamples(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		srcESLint:              destESLint,
		"eslint/node/nvm/pnpm": destESLint,
		"eslint/node/fnm/npm":  destESLint,
		"eslint/node/nvm/npm":  destESLint,
		"eslint/node/fnm/yarn": destESLint,
		"eslint/node/nvm/yarn": destESLint,
		srcESLintBun:           destESLint,
		"pnpm/fnm":             destPnpm,
		"npm/nvm":              destNpm,
		"yarn/fnm":             destYarn,
		"corepack/fnm":         destCorepack,
		"corepack/nvm":         destCorepack,
		"eslint-pnpm-fnm":      destESLint,
		"eslint-pnpm-nvm":      destESLint,
		"eslint-npm-fnm":       destESLint,
		"eslint-npm-nvm":       destESLint,
		"eslint-yarn-fnm":      destESLint,
		"eslint-yarn-nvm":      destESLint,
		"eslint-bun":           destESLint,
		"pnpm-fnm":             destPnpm,
		"npm-nvm":              destNpm,
		"yarn-fnm":             destYarn,
		"corepack-fnm":         destCorepack,
		"corepack-nvm":         destCorepack,
		destFnm:                destFnm,
		destNvm:                destNvm,
		destBun:                destBun,
		consts.Go:              consts.Go,
	}

	for source := range cases {
		want := cases[source]

		got, err := normalizer.Normalize(source)
		if err != nil {
			t.Fatalf("Normalize(%q) error = %v", source, err)
		}

		if got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", source, got, want)
		}
	}
}

// TestLongestSuffixFirst verifies the longest matching suffix is normalized first.
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

// TestNormalizeRejectsEmptySource verifies an empty source name returns an error.
func TestNormalizeRejectsEmptySource(t *testing.T) {
	t.Parallel()

	_, err := normalizer.Normalize("")
	if err == nil {
		t.Fatal("expected empty source error")
	}
}

// TestBuildDestinationMapPropagatesNormalizeError verifies a normalize failure propagates from the map builder.
func TestBuildDestinationMapPropagatesNormalizeError(t *testing.T) {
	t.Parallel()

	_, err := normalizer.BuildDestinationMap([]string{""})
	if err == nil {
		t.Fatal("expected normalize error")
	}
}

// TestDestinationCollision verifies two sources normalizing to the same destination error.
func TestDestinationCollision(t *testing.T) {
	t.Parallel()

	_, err := normalizer.BuildDestinationMap([]string{srcESLint, srcESLintBun})
	if err == nil {
		t.Fatal("expected collision error")
	}

	if !strings.Contains(err.Error(), "Destination collision") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestBuildDestinationMapSortsSources verifies sources are returned sorted by destination.
func TestBuildDestinationMapSortsSources(t *testing.T) {
	t.Parallel()

	mapping, err := normalizer.BuildDestinationMap([]string{consts.Go, srcESLint})
	if err != nil {
		t.Fatal(err)
	}

	got := normalizer.SortedSources(mapping)

	want := []string{srcESLint, consts.Go}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedSources() = %#v, want %#v", got, want)
		}
	}
}
