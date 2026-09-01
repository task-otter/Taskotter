// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/resolve/service"
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
