// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/resolve/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	resolveInputParams struct {
		task           string
		cat            map[string]struct{}
		packageManager config.PackageManager
		versionManager config.VersionManager
	}

	nodeVariantExpect struct {
		cat            map[string]struct{}
		packageManager config.PackageManager
		versionManager config.VersionManager
		want           string
	}

	nodeConfigErrorCase struct {
		name    string
		pm      config.PackageManager
		vm      config.VersionManager
		wantMsg string
		catalog []string
	}

	nodeVariantWant struct {
		pm   config.PackageManager
		vm   config.VersionManager
		want string
	}
)

const (
	taskPrettier = "prettier"

	moduleEslintFnmNpm = "eslint/node/fnm/npm"

	moduleEslintNvmNpm = "eslint/node/nvm/npm"

	moduleEslintFnmYarn = "eslint/node/fnm/yarn"

	moduleEslintNvmYarn = "eslint/node/nvm/yarn"

	moduleEslintNvmPnpm = "eslint/node/nvm/pnpm"

	errExpected = "expected error"
)

func nodeConfigurationErrorCases() []nodeConfigErrorCase {
	return []nodeConfigErrorCase{
		{
			name:    "requires package manager",
			catalog: []string{srcESLintBun},
			pm:      consts.Empty,
			vm:      consts.Empty,
			wantMsg: "requires js configuration",
		},
		{
			name:    "requires version manager",
			catalog: []string{moduleEslintFnmNpm},
			pm:      config.PMNPM,
			vm:      consts.Empty,
			wantMsg: "js.version-manager required",
		},
	}
}

// TestMissingTaskCloseMatches verifies a misspelled task name returns close match suggestions.
func TestMissingTaskCloseMatches(t *testing.T) {
	t.Parallel()

	err := resolveExpectErr(
		t,
		resolveInput(&resolveInputParams{
			task:           "eslit",
			cat:            catalog(srcESLintBun, moduleEslintFnmNpm),
			packageManager: config.PackageManager(config.JSRuntimeBun),
			versionManager: consts.Empty,
		}),
	)
	if err == nil {
		t.Fatal(errExpected)
	}

	assertHasCloseMatches(t, err)
}

// TestMissingTaskWithoutCloseMatches verifies an unrelated missing task returns no close matches.
func TestMissingTaskWithoutCloseMatches(t *testing.T) {
	t.Parallel()

	err := resolveExpectErr(
		t,
		resolveInput(&resolveInputParams{
			task:           "zzz",
			cat:            catalog(consts.Go),
			packageManager: consts.Empty,
			versionManager: consts.Empty,
		}),
	)
	if err == nil {
		t.Fatal("expected missing task error")
	}

	if strings.Contains(err.Error(), "close matches") {
		t.Fatalf("unexpected close matches: %v", err)
	}
}

// TestNodeAttemptedSourceMissing verifies the error names the attempted source module when missing.
func TestNodeAttemptedSourceMissing(t *testing.T) {
	t.Parallel()

	err := resolveExpectErr(
		t,
		resolveInput(&resolveInputParams{
			task:           taskESLint,
			cat:            catalog(moduleEslintFnmNpm),
			packageManager: config.PMPnpm,
			versionManager: config.VMFnm,
		}),
	)
	if err == nil {
		t.Fatal("expected missing attempted source error")
	}

	if !strings.Contains(err.Error(), `attempted source module "eslint/node/fnm/pnpm"`) {
		t.Fatalf(fmtUnexpectedErr, err)
	}
}

// TestResolveAll verifies multiple logical tasks resolve into the expected number of resolutions.
func TestResolveAll(t *testing.T) {
	t.Parallel()

	resolutions, err := service.ResolveAll(&service.ResolveAllInput{
		Tasks:          []string{consts.Go, taskPrettier},
		Catalog:        catalog(consts.Go, taskPrettier),
		PackageManager: consts.Empty,
		VersionManager: consts.Empty,
	})
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

	resolutions, err := service.ResolveAll(&service.ResolveAllInput{
		Tasks:          []string{consts.Go, missingModule},
		Catalog:        catalog(consts.Go),
		PackageManager: consts.Empty,
		VersionManager: consts.Empty,
	})
	iox.Discard(resolutions)

	if err == nil {
		t.Fatal("expected ResolveAll error")
	}
}

// TestResolveInvalidPackageManager verifies an unrecognized package manager value is rejected.
func TestResolveInvalidPackageManager(t *testing.T) {
	t.Parallel()

	err := resolveExpectErr(
		t,
		resolveInput(&resolveInputParams{
			task:           taskESLint,
			cat:            catalog(moduleEslintFnmNpm),
			packageManager: config.PackageManager(pkgDeno),
			versionManager: consts.Empty,
		}),
	)
	if err == nil {
		t.Fatal("expected invalid package manager error")
	}
}

// TestResolveNodeConfigurationErrors verifies node tasks fail without required JS settings.
func TestResolveNodeConfigurationErrors(t *testing.T) {
	t.Parallel()

	cases := nodeConfigurationErrorCases()

	for idx := range cases {
		testCase := cases[idx]

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertNodeConfigurationError(t, &testCase)
		})
	}
}

// TestResolveNodeVariants verifies node tasks resolve to the module matching the package/version manager.
func TestResolveNodeVariants(t *testing.T) {
	t.Parallel()
	runNodeVariantCases(t, nodeVariantCatalog())
}

// TestResolveNonNodeTask verifies a non-node task resolves directly to its module.
func TestResolveNonNodeTask(t *testing.T) {
	t.Parallel()

	res, err := service.Resolve(
		resolveInput(&resolveInputParams{
			task:           consts.Go,
			cat:            catalog(consts.Go),
			packageManager: consts.Empty,
			versionManager: consts.Empty,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if res.SourceModule != consts.Go {
		t.Fatalf(fmtGotQ, res.SourceModule)
	}
}

func assertHasCloseMatches(t *testing.T, err error) {
	t.Helper()

	resolveErr := &service.ResolveError{
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

func assertNodeConfigurationError(t *testing.T, testCase *nodeConfigErrorCase) {
	t.Helper()

	err := resolveExpectErr(t, resolveInput(&resolveInputParams{
		task:           taskESLint,
		cat:            catalog(testCase.catalog...),
		packageManager: testCase.pm,
		versionManager: testCase.vm,
	}))
	if err == nil {
		t.Fatal(errExpected)
	}

	if !strings.Contains(err.Error(), testCase.wantMsg) {
		t.Fatalf(fmtUnexpectedErr, err)
	}
}

func assertNodeVariantResolved(t *testing.T, expect *nodeVariantExpect) {
	t.Helper()

	res, err := service.Resolve(resolveInput(&resolveInputParams{
		task:           taskESLint,
		cat:            expect.cat,
		packageManager: expect.packageManager,
		versionManager: expect.versionManager,
	}))
	if err != nil {
		t.Fatal(err)
	}

	if res.SourceModule != expect.want {
		t.Fatalf("got %q, want %q", res.SourceModule, expect.want)
	}
}

func catalog(names ...string) map[string]struct{} {
	cat := make(map[string]struct{}, len(names))

	for idx := range names {
		cat[names[idx]] = struct{}{}
	}

	return cat
}

func nodeVariantCatalog() map[string]struct{} {
	return catalog(
		taskESLint,
		moduleEslintFnmNpm, moduleEslintNvmNpm,
		moduleEslintFnmYarn, moduleEslintNvmYarn,
		srcESLintFnmPnpm, moduleEslintNvmPnpm,
		srcESLintBun,
	)
}

func nodeVariantWantRows() []nodeVariantWant {
	return []nodeVariantWant{
		{config.PMNPM, config.VMFnm, moduleEslintFnmNpm},
		{config.PMNPM, config.VMNvm, moduleEslintNvmNpm},
		{config.PMYarn, config.VMFnm, moduleEslintFnmYarn},
		{config.PMYarn, config.VMNvm, moduleEslintNvmYarn},
		{config.PMPnpm, config.VMFnm, srcESLintFnmPnpm},
		{config.PMPnpm, config.VMNvm, moduleEslintNvmPnpm},
		{config.PackageManager(config.JSRuntimeBun), "", srcESLintBun},
	}
}

func resolveExpectErr(t *testing.T, input *service.ResolveInput) error {
	t.Helper()

	res, err := service.Resolve(input)
	iox.Discard(res)

	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}

	return nil
}

func resolveInput(p *resolveInputParams) *service.ResolveInput {
	return &service.ResolveInput{
		Task:           p.task,
		Catalog:        p.cat,
		PackageManager: p.packageManager,
		VersionManager: p.versionManager,
	}
}

func runNodeVariantCases(t *testing.T, cat map[string]struct{}) {
	t.Helper()

	rows := nodeVariantWantRows()

	for i := range rows {
		row := rows[i]
		assertNodeVariantResolved(t, &nodeVariantExpect{
			cat:            cat,
			packageManager: row.pm,
			versionManager: row.vm,
			want:           row.want,
		})
	}
}
