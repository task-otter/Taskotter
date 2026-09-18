// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package dependency

import (
	"errors"
	"testing"
)

const (
	dependencyApp     = "app"
	dependencyCore    = "core"
	dependencyMissing = "missing"
	dependencyShared  = "shared"
)

func TestResolveTransitiveGraph(t *testing.T) {
	t.Parallel()

	got, err := ResolveTransitive([]string{dependencyApp}, map[string][]string{
		dependencyApp:    {dependencyCore, dependencyShared},
		dependencyCore:   {dependencyShared},
		dependencyShared: {},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0] != "core" || got[1] != "shared" {
		t.Fatalf("dependencies = %#v", got)
	}
}

func TestTransitiveResolverWrapsError(t *testing.T) {
	t.Parallel()

	_, err := (transitiveResolver{}).Resolve([]string{dependencyMissing}, map[string][]string{})

	if err == nil || !errors.Is(err, errModuleNotDefined) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveTransitiveErrors(t *testing.T) {
	t.Parallel()

	t.Run("missing requested", func(t *testing.T) {
		t.Parallel()

		_, err := ResolveTransitive([]string{dependencyMissing}, map[string][]string{})

		if err == nil || !errors.Is(err, errModuleNotDefined) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("missing dependency", func(t *testing.T) {
		t.Parallel()

		_, err := ResolveTransitive(
			[]string{dependencyApp},
			map[string][]string{dependencyApp: {dependencyMissing}},
		)

		var missing *MissingDependencyError

		if !errors.As(err, &missing) || missing.Module != "app" || missing.Dependency != "missing" {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		t.Parallel()

		_, err := ResolveTransitive([]string{"a"}, map[string][]string{"a": {"b"}, "b": {"a"}})

		var cycle *CycleError

		if !errors.As(err, &cycle) || len(cycle.Path) != 3 {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDependencyErrors(t *testing.T) {
	t.Parallel()

	if (&CycleError{Path: []string{"a", "b", "a"}}).Error() != "dependency cycle detected: a -> b -> a" {
		t.Fatal("unexpected cycle error")
	}

	if (&MissingDependencyError{Module: "a", Dependency: "b"}).Error() != "module \"a\" depends on missing module \"b\"" {
		t.Fatal("unexpected missing dependency error")
	}
}
