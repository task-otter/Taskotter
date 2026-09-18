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

const (
	modNodejs  = "nodejs"
	modNix     = "nix"
	fmtGotWant = "got %#v, want %#v"
	nodeA      = "a"
	nodeB      = "b"
	nodeC      = "c"
)

func deps() map[string][]string {
	return map[string][]string{
		srcESLintPnpm: {destPnpm},
		destPnpm:      {modNodejs, modNix},
		modNodejs:     {modNix},
		modNix:        {},
		"go":          {},
	}
}

// TestResolveTransitive verifies transitive dependencies resolve in dependency order.
func TestResolveTransitive(t *testing.T) {
	t.Parallel()

	got, err := service.ResolveTransitive([]string{srcESLintPnpm}, deps())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{modNix, modNodejs, destPnpm}

	if len(got) != len(want) {
		t.Fatalf(fmtGotWant, got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf(fmtGotWant, got, want)
		}
	}
}

// TestDuplicateDependencyDeduped verifies a module already requested is excluded from its own deps.
func TestDuplicateDependencyDeduped(t *testing.T) {
	t.Parallel()

	got, err := service.ResolveTransitive([]string{srcESLintPnpm, destPnpm}, deps())
	if err != nil {
		t.Fatal(err)
	}

	for i := range got {
		if got[i] == destPnpm {
			t.Fatal("requested module should not appear in dependencies")
		}
	}
}

// TestMissingDependency verifies a dependency missing from .deps.yml returns an error.
func TestMissingDependncy(t *testing.T) {
	t.Parallel()

	depMap := deps()

	depMap[srcESLintPnpm] = []string{"missing-mod"}

	got, err := service.ResolveTransitive([]string{srcESLintPnpm}, depMap)
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected missing dependency error")
	}

	want := `module "eslint/node/pnpm" depends on missing module "missing-mod"`

	if !strings.Contains(err.Error(), want) {
		t.Fatalf(fmtUnexpectedErr, err)
	}
}

// TestDependencyCycle verifies a circular dependency chain returns a cycle error.
func TestDependencyCycle(t *testing.T) {
	t.Parallel()

	depMap := map[string][]string{
		nodeA: {nodeB},
		nodeB: {nodeC},
		nodeC: {nodeA},
	}

	got, err := service.ResolveTransitive([]string{nodeA}, depMap)
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected cycle error")
	}

	if !strings.Contains(err.Error(), "a -> b -> c -> a") {
		t.Fatalf("cycle error = %v", err)
	}
}

// TestRequestedModuleMissingFromDependencyFile verifies a requested module absent from the file errors.
func TestRequestedModuleMissingFromDependencyFile(t *testing.T) {
	t.Parallel()

	got, err := service.ResolveTransitive([]string{missingModule}, deps())
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected missing requested module error")
	}
}

func eslintNormalizeCases() map[string]string {
	return map[string]string{
		srcESLintPnpm:       taskESLint,
		"eslint/node/npm":   taskESLint,
		"eslint/node/yarn":  taskESLint,
		srcESLintBun:        taskESLint,
		"prettier/node/npm": taskPrettier,
	}
}

// toolNormalizeCases covers flat store modules, which normalize to themselves
// now that the store has no version-manager variant directories.
func toolNormalizeCases() map[string]string {
	return map[string]string{
		destPnpm:  destPnpm,
		destNpm:   destNpm,
		destYarn:  destYarn,
		destBun:   destBun,
		consts.Go: consts.Go,
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

	got, err := service.Normalize(srcESLintPnpm)
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

	got, err := service.BuildDestinationMap([]string{srcESLintPnpm, srcESLintBun})
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

	mapping, err := service.BuildDestinationMap([]string{consts.Go, srcESLintPnpm})
	if err != nil {
		t.Fatal(err)
	}

	got := service.SortedSources(mapping)

	want := []string{srcESLintPnpm, consts.Go}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedSources() = %#v, want %#v", got, want)
		}
	}
}

func nodeConfigurationErrorCases() []nodeConfigErrorCase {
	return []nodeConfigErrorCase{
		{
			name:    "requires package manager",
			catalog: []string{srcESLintBun},
			pm:      consts.Empty,
			wantMsg: "requires js configuration",
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
			cat:            catalog(srcESLintBun, moduleEslintNpm),
			packageManager: config.JSRuntimeBun,
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
			cat:            catalog(moduleEslintNpm),
			packageManager: config.PMPnpm,
		}),
	)
	if err == nil {
		t.Fatal("expected missing attempted source error")
	}

	if !strings.Contains(err.Error(), `attempted source module "eslint/node/pnpm"`) {
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
			cat:            catalog(moduleEslintNpm),
			packageManager: config.PackageManager(pkgDeno),
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

// TestResolveNodeVariants verifies node tasks resolve to the module matching the package manager.
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
		moduleEslintNpm,
		moduleEslintYarn,
		srcESLintPnpm,
		srcESLintBun,
	)
}

func nodeVariantWantRows() []nodeVariantWant {
	return []nodeVariantWant{
		{config.PMNPM, moduleEslintNpm},
		{config.PMYarn, moduleEslintYarn},
		{config.PMPnpm, srcESLintPnpm},
		{config.JSRuntimeBun, srcESLintBun},
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
			want:           row.want,
		})
	}
}

func assertBuildSourceModule(t *testing.T, testCase *buildSourceModuleCase) {
	t.Helper()

	got, err := service.BuildSourceModule(testCase.task, testCase.pkgMgr)
	if err != nil {
		t.Fatal(err)
	}

	if got != testCase.want {
		t.Fatalf(fmtGotQ, got)
	}
}

func assertBuildSourceModuleError(t *testing.T, testCase *buildSourceModuleCase) {
	t.Helper()

	got, err := service.BuildSourceModule(testCase.task, testCase.pkgMgr)
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected BuildSourceModule error")
	}
}

// TestBuildSourceModule verifies source module names build correctly and reject invalid combinations.
func TestBuildSourceModule(t *testing.T) {
	t.Parallel()

	assertBuildSourceModule(t, &buildSourceModuleCase{
		task: taskESLint, pkgMgr: config.PMPnpm, want: srcESLintPnpm,
	})
	assertBuildSourceModule(t, &buildSourceModuleCase{
		task:   taskESLint,
		pkgMgr: config.JSRuntimeBun,
		want:   srcESLintBun,
	})
	assertBuildSourceModuleError(t, &buildSourceModuleCase{
		task:   taskESLint,
		pkgMgr: config.PackageManager(pkgDeno),
		want:   consts.Empty,
	})
}

func nodeToolVariantCases() []nodeToolVariantCase {
	return []nodeToolVariantCase{
		{moduleName: srcESLintPnpm, logicalTask: taskESLint, expected: true},
		{moduleName: srcESLintBun, logicalTask: taskESLint, expected: true},
		{moduleName: consts.Go, logicalTask: consts.Go, expected: false},
		{moduleName: taskESLint, logicalTask: taskESLint, expected: false},
		{moduleName: "prettier/node/pnpm", logicalTask: taskESLint, expected: false},
		{moduleName: "eslint/node/deno", logicalTask: taskESLint, expected: false},
		{moduleName: "eslint/node/fnm/pnpm", logicalTask: taskESLint, expected: false},
	}
}

func assertNodeToolVariant(t *testing.T, testCase *nodeToolVariantCase) {
	t.Helper()

	got := service.IsNodeToolVariant(testCase.moduleName, testCase.logicalTask)

	if got != testCase.expected {
		t.Fatalf(
			"IsNodeToolVariant(%q, %q) = %t, want %t",
			testCase.moduleName,
			testCase.logicalTask,
			got,
			testCase.expected,
		)
	}
}

// TestIsNodeToolVariant verifies node tool variant module names are correctly identified.
func TestIsNodeToolVariant(t *testing.T) {
	t.Parallel()

	cases := nodeToolVariantCases()

	for i := range cases {
		assertNodeToolVariant(t, &cases[i])
	}
}

func stripOneSuffixPrimaryCases() []stripOneSuffixCase {
	return []stripOneSuffixCase{
		{input: srcESLintPnpm, wantStripped: true, wantResult: taskESLint},
		{input: srcESLintBun, wantStripped: true, wantResult: taskESLint},
		{input: taskESLint, wantStripped: false, wantResult: taskESLint},
		{input: destPnpm, wantStripped: false, wantResult: destPnpm},
	}
}

func stripOneSuffixEdgeCases() []stripOneSuffixCase {
	return []stripOneSuffixCase{
		{input: suffixBun, wantStripped: false, wantResult: suffixBun},
	}
}

func assertStripOneSuffix(t *testing.T, testCase *stripOneSuffixCase) {
	t.Helper()

	got, stripped := service.StripOneSuffix(testCase.input)

	if stripped != testCase.wantStripped || got != testCase.wantResult {
		t.Fatalf(fmtGotStrippedQ, got, stripped)
	}
}

func assertStripOneSuffixCases(t *testing.T, cases []stripOneSuffixCase) {
	t.Helper()

	for i := range cases {
		assertStripOneSuffix(t, &cases[i])
	}
}

// TestStripOneSuffix verifies a single matching suffix is stripped from module names.
func TestStripOneSuffix(t *testing.T) {
	t.Parallel()

	assertStripOneSuffixCases(t, stripOneSuffixPrimaryCases())
	assertStripOneSuffixCases(t, stripOneSuffixEdgeCases())
}
