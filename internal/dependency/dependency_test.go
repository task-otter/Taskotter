package dependency_test

import (
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/dependency"
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
}
