// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package resolver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/resolver"
)

const (
	taskEslint          = "eslint"
	taskPrettier        = "prettier"
	moduleEslintBun     = "eslint/bun"
	moduleEslintFnmNpm  = "eslint/node/fnm/npm"
	moduleEslintNvmNpm  = "eslint/node/nvm/npm"
	moduleEslintFnmYarn = "eslint/node/fnm/yarn"
	moduleEslintNvmYarn = "eslint/node/nvm/yarn"
	moduleEslintFnmPnpm = "eslint/node/fnm/pnpm"
	moduleEslintNvmPnpm = "eslint/node/nvm/pnpm"
	errExpected         = "expected error"
	fmtUnexpectedErr    = "unexpected error: %v"
)

func catalog(names ...string) map[string]struct{} {
	cat := make(map[string]struct{}, len(names))

	for i := range names {
		cat[names[i]] = struct{}{}
	}

	return cat
}

// TestResolveNonNodeTask verifies a non-node task resolves directly to its module.
func TestResolveNonNodeTask(t *testing.T) {
	t.Parallel()

	res, err := resolver.Resolve(consts.Go, catalog(consts.Go), consts.Empty, consts.Empty)
	if err != nil {
		t.Fatal(err)
	}

	if res.SourceModule != consts.Go {
		t.Fatalf("got %q", res.SourceModule)
	}
}

// TestResolveAll verifies multiple logical tasks resolve into the expected number of resolutions.
func TestResolveAll(t *testing.T) {
	t.Parallel()

	resolutions, err := resolver.ResolveAll(
		[]string{consts.Go, taskPrettier},
		catalog(consts.Go, taskPrettier),
		consts.Empty,
		consts.Empty,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(resolutions) != consts.IndexTwo {
		t.Fatalf("ResolveAll() returned %d resolutions", len(resolutions))
	}
}

// TestResolveAllStopsOnError verifies resolution stops and errors on the first missing task.
func TestResolveAllStopsOnError(t *testing.T) {
	t.Parallel()

	_, err := resolver.ResolveAll(
		[]string{consts.Go, "missing"},
		catalog(consts.Go),
		consts.Empty,
		consts.Empty,
	)
	if err == nil {
		t.Fatal("expected ResolveAll error")
	}
}

// TestResolveNodeVariants verifies node tasks resolve to the module matching the package/version manager.
func TestResolveNodeVariants(t *testing.T) {
	t.Parallel()

	cat := catalog(
		taskEslint,
		moduleEslintFnmNpm, moduleEslintNvmNpm,
		moduleEslintFnmYarn, moduleEslintNvmYarn,
		moduleEslintFnmPnpm, moduleEslintNvmPnpm,
		moduleEslintBun,
	)

	cases := []struct {
		pm   config.PackageManager
		vm   config.VersionManager
		want string
	}{
		{config.PMNPM, config.VMFnm, moduleEslintFnmNpm},
		{config.PMNPM, config.VMNvm, moduleEslintNvmNpm},
		{config.PMYarn, config.VMFnm, moduleEslintFnmYarn},
		{config.PMYarn, config.VMNvm, moduleEslintNvmYarn},
		{config.PMPnpm, config.VMFnm, moduleEslintFnmPnpm},
		{config.PMPnpm, config.VMNvm, moduleEslintNvmPnpm},
		{config.PMBun, consts.Empty, moduleEslintBun},
	}

	for i := range cases {
		testCase := &cases[i]

		res, err := resolver.Resolve(taskEslint, cat, testCase.pm, testCase.vm)
		if err != nil {
			t.Fatalf("%+v: %v", testCase, err)
		}

		if res.SourceModule != testCase.want {
			t.Fatalf("%+v: got %q", testCase, res.SourceModule)
		}
	}
}

// TestNodeTaskRequiresPackageManager verifies a node task without js configuration fails.
func TestNodeTaskRequiresPackageManager(t *testing.T) {
	t.Parallel()

	cat := catalog(moduleEslintBun)

	_, err := resolver.Resolve(taskEslint, cat, consts.Empty, consts.Empty)
	if err == nil {
		t.Fatal(errExpected)
	}

	if !strings.Contains(err.Error(), "requires js configuration") {
		t.Fatalf(fmtUnexpectedErr, err)
	}
}

// TestNpmRequiresVersionManager verifies npm package manager requires a version manager to be set.
func TestNpmRequiresVersionManager(t *testing.T) {
	t.Parallel()

	cat := catalog(moduleEslintFnmNpm)

	_, err := resolver.Resolve(taskEslint, cat, config.PMNPM, consts.Empty)
	if err == nil {
		t.Fatal(errExpected)
	}

	if !strings.Contains(err.Error(), "js.version-manager required") {
		t.Fatalf(fmtUnexpectedErr, err)
	}
}

// TestNodeAttemptedSourceMissing verifies the error names the attempted source module when missing.
func TestNodeAttemptedSourceMissing(t *testing.T) {
	t.Parallel()

	cat := catalog(moduleEslintFnmNpm)

	_, err := resolver.Resolve(taskEslint, cat, config.PMPnpm, config.VMFnm)
	if err == nil {
		t.Fatal("expected missing attempted source error")
	}

	if !strings.Contains(err.Error(), `attempted source module "eslint/node/fnm/pnpm"`) {
		t.Fatalf(fmtUnexpectedErr, err)
	}
}

// TestResolveInvalidPackageManager verifies an unrecognized package manager value is rejected.
func TestResolveInvalidPackageManager(t *testing.T) {
	t.Parallel()

	_, err := resolver.Resolve(
		taskEslint,
		catalog(moduleEslintFnmNpm),
		config.PackageManager("deno"),
		"",
	)
	if err == nil {
		t.Fatal("expected invalid package manager error")
	}
}

// TestMissingTaskCloseMatches verifies a misspelled task name returns close match suggestions.
func TestMissingTaskCloseMatches(t *testing.T) {
	t.Parallel()

	cat := catalog(moduleEslintBun, moduleEslintFnmNpm)

	_, err := resolver.Resolve("eslit", cat, config.PMBun, consts.Empty)
	if err == nil {
		t.Fatal(errExpected)
	}

	assertHasCloseMatches(t, err)
}

func assertHasCloseMatches(t *testing.T, err error) {
	t.Helper()

	resolveErr := &resolver.ResolveError{
		LogicalTask:  consts.Empty,
		Attempted:    consts.Empty,
		Message:      consts.Empty,
		CloseMatches: nil,
	}

	ok := errors.As(err, &resolveErr)

	if !ok {
		t.Fatalf("unexpected error type: %T", err)
	}

	if len(resolveErr.CloseMatches) == consts.IndexZero {
		t.Fatal("expected close matches")
	}
}

// TestMissingTaskWithoutCloseMatches verifies an unrelated missing task returns no close matches.
func TestMissingTaskWithoutCloseMatches(t *testing.T) {
	t.Parallel()

	_, err := resolver.Resolve("zzz", catalog(consts.Go), consts.Empty, consts.Empty)
	if err == nil {
		t.Fatal("expected missing task error")
	}

	if strings.Contains(err.Error(), "close matches") {
		t.Fatalf("unexpected close matches: %v", err)
	}
}
