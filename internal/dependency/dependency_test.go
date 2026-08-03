// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package dependency_test

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/dependency"
)

const (
	modESLintPnpmFnm = "eslint/node/fnm/pnpm"
	modPnpmFnm       = "pnpm/fnm"
	modCorepackFnm   = "corepack/fnm"
	modFnm           = "fnm"
	fmtGotWant       = "got %#v, want %#v"
	nodeA            = "a"
	nodeB            = "b"
	nodeC            = "c"
)

func deps() map[string][]string {
	return map[string][]string{
		modESLintPnpmFnm: {modPnpmFnm},
		modPnpmFnm:       {modCorepackFnm, modFnm},
		modCorepackFnm:   {modFnm},
		modFnm:           {},
		"go":             {},
	}
}

// TestResolveTransitive verifies transitive dependencies resolve in dependency order.
func TestResolveTransitive(t *testing.T) {
	t.Parallel()

	got, err := dependency.ResolveTransitive([]string{modESLintPnpmFnm}, deps())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{modCorepackFnm, modFnm, modPnpmFnm}

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

	got, err := dependency.ResolveTransitive([]string{modESLintPnpmFnm, modPnpmFnm}, deps())
	if err != nil {
		t.Fatal(err)
	}

	for i := range got {
		if got[i] == modPnpmFnm {
			t.Fatal("requested module should not appear in dependencies")
		}
	}
}

// TestMissingDependency verifies a dependency missing from .deps.yml returns an error.
func TestMissingDependncy(t *testing.T) {
	t.Parallel()

	depMap := deps()

	depMap[modESLintPnpmFnm] = []string{"missing-mod"}

	_, err := dependency.ResolveTransitive([]string{modESLintPnpmFnm}, depMap)
	if err == nil {
		t.Fatal("expected missing dependency error")
	}

	if !strings.Contains(
		err.Error(),
		`module "eslint/node/fnm/pnpm" depends on missing module "missing-mod"`,
	) {
		t.Fatalf("unexpected error: %v", err)
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

	_, err := dependency.ResolveTransitive([]string{nodeA}, depMap)
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

	_, err := dependency.ResolveTransitive([]string{"missing"}, deps())
	if err == nil {
		t.Fatal("expected missing requested module error")
	}
}
