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

func TestResolveTransitive(t *testing.T) {
	t.Parallel()

	got, err := dependency.ResolveTransitive([]string{modESLintPnpmFnm}, deps())
	if err != nil {
		t.Fatal(err)
	}

	want := []string{modCorepackFnm, modFnm, modPnpmFnm}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestDuplicateDependencyDeduped(t *testing.T) {
	t.Parallel()

	got, err := dependency.ResolveTransitive([]string{modESLintPnpmFnm, modPnpmFnm}, deps())
	if err != nil {
		t.Fatal(err)
	}

	for _, dep := range got {
		if dep == modPnpmFnm {
			t.Fatal("requested module should not appear in dependencies")
		}
	}
}

func TestMissingDependency(t *testing.T) {
	t.Parallel()

	depMap := deps()
	depMap[modESLintPnpmFnm] = []string{"missing-mod"}

	_, err := dependency.ResolveTransitive([]string{modESLintPnpmFnm}, depMap)
	if err == nil {
		t.Fatal("expected missing dependency error")
	}

	if !strings.Contains(err.Error(), `module "eslint/node/fnm/pnpm" depends on missing module "missing-mod"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDependencyCycle(t *testing.T) {
	t.Parallel()

	depMap := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {"a"},
	}

	_, err := dependency.ResolveTransitive([]string{"a"}, depMap)
	if err == nil {
		t.Fatal("expected cycle error")
	}

	if !strings.Contains(err.Error(), "a -> b -> c -> a") {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestRequestedModuleMissingFromDependencyFile(t *testing.T) {
	t.Parallel()

	_, err := dependency.ResolveTransitive([]string{"missing"}, deps())
	if err == nil {
		t.Fatal("expected missing requested module error")
	}
}
