// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package normalize

import (
	"errors"
	"testing"
)

func TestNormalizeAndSortedSources(t *testing.T) {
	t.Parallel()

	got, err := Normalize("lint/node/pnpm/bun")

	if err != nil || got != "lint" {
		t.Fatalf("Normalize() = %q, %v", got, err)
	}

	if _, err := Normalize(""); err == nil {
		t.Fatal("empty normalized name accepted")
	} else if !errors.Is(err, errEmptyNormalizedName) {
		t.Fatalf("err = %v", err)
	}

	sources := map[string]string{"z": "z", "a": "a"}

	if got := SortedSources(sources); len(got) != 2 || got[0] != "a" || got[1] != "z" {
		t.Fatalf("SortedSources() = %#v", got)
	}
}

func TestBuildDestinationMapCollision(t *testing.T) {
	t.Parallel()

	got, err := BuildDestinationMap([]string{"lint/node/npm", "format/bun"})

	if err != nil || got["lint/node/npm"] != "lint" || got["format/bun"] != "format" {
		t.Fatalf("map = %#v, err = %v", got, err)
	}

	_, err = BuildDestinationMap([]string{"lint/node/npm", "lint/bun"})

	if _, ok := errors.AsType[*CollisionError](err); !ok {
		t.Fatalf("err = %v", err)
	}

	if _, err := BuildDestinationMap([]string{""}); err == nil {
		t.Fatal("normalization error ignored")
	}
}
