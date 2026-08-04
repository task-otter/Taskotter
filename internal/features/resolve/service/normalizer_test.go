// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	destCorepack = "corepack"
	destPnpm     = "pnpm"
	destNpm      = "npm"
	destYarn     = "yarn"
	destNvm      = "nvm"
	destBun      = "bun"
)

func eslintNormalizeCases() map[string]string {
	return map[string]string{
		srcESLintFnmPnpm:       taskESLint,
		"eslint/node/nvm/pnpm": taskESLint,
		"eslint/node/fnm/npm":  taskESLint,
		"eslint/node/nvm/npm":  taskESLint,
		"eslint/node/fnm/yarn": taskESLint,
		"eslint/node/nvm/yarn": taskESLint,
		srcESLintBun:           taskESLint,
		"eslint-pnpm-fnm":      taskESLint,
		"eslint-pnpm-nvm":      taskESLint,
		"eslint-npm-fnm":       taskESLint,
		"eslint-npm-nvm":       taskESLint,
		"eslint-yarn-fnm":      taskESLint,
		"eslint-yarn-nvm":      taskESLint,
		"eslint-bun":           taskESLint,
	}
}

func toolNormalizeCases() map[string]string {
	return map[string]string{
		"pnpm/fnm":     destPnpm,
		"npm/nvm":      destNpm,
		"yarn/fnm":     destYarn,
		"corepack/fnm": destCorepack,
		"corepack/nvm": destCorepack,
		"pnpm-fnm":     destPnpm,
		"npm-nvm":      destNpm,
		"yarn-fnm":     destYarn,
		"corepack-fnm": destCorepack,
		"corepack-nvm": destCorepack,
		modFnm:         modFnm,
		destNvm:        destNvm,
		destBun:        destBun,
		consts.Go:      consts.Go,
	}
}

func assertNormalize(t *testing.T, source, want string) {
	t.Helper()

	got, err := service.Normalize(source)
	if err != nil {
		t.Fatalf("Normalize(%q) error = %v", source, err)
	}

	if got != want {
		t.Fatalf("Normalize(%q) = %q, want %q", source, got, want)
	}
}

func assertNormalizeCases(t *testing.T, cases map[string]string) {
	t.Helper()

	for source := range cases {
		assertNormalize(t, source, cases[source])
	}
}

// TestNormalizeExamples verifies variant module source names normalize to expected destinations.
func TestNormalizeExamples(t *testing.T) {
	t.Parallel()

	assertNormalizeCases(t, eslintNormalizeCases())
	assertNormalizeCases(t, toolNormalizeCases())
}

// TestLongestSuffixFirst verifies the longest matching suffix is normalized first.
func TestLongestSuffixFirst(t *testing.T) {
	t.Parallel()

	got, err := service.Normalize(srcESLintFnmPnpm)
	if err != nil {
		t.Fatal(err)
	}

	if got != taskESLint {
		t.Fatalf(fmtGotQ, got)
	}
}

// TestNormalizeRejectsEmptySource verifies an empty source name returns an error.
func TestNormalizeRejectsEmptySource(t *testing.T) {
	t.Parallel()

	got, err := service.Normalize("")
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected empty source error")
	}
}

// TestBuildDestinationMapPropagatesNormalizeError verifies a normalize failure propagates from the map builder.
func TestBuildDestinationMapPropagatesNormalizeError(t *testing.T) {
	t.Parallel()

	got, err := service.BuildDestinationMap([]string{""})
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected normalize error")
	}
}

// TestDestinationCollision verifies two sources normalizing to the same destination error.
func TestDestinationCollision(t *testing.T) {
	t.Parallel()

	got, err := service.BuildDestinationMap([]string{srcESLintFnmPnpm, srcESLintBun})
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected collision error")
	}

	if !strings.Contains(err.Error(), "Destination collision") {
		t.Fatalf(fmtUnexpectedErr, err)
	}
}

// TestBuildDestinationMapSortsSources verifies sources are returned sorted by destination.
func TestBuildDestinationMapSortsSources(t *testing.T) {
	t.Parallel()

	mapping, err := service.BuildDestinationMap([]string{consts.Go, srcESLintFnmPnpm})
	if err != nil {
		t.Fatal(err)
	}

	got := service.SortedSources(mapping)

	want := []string{srcESLintFnmPnpm, consts.Go}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedSources() = %#v, want %#v", got, want)
		}
	}
}
