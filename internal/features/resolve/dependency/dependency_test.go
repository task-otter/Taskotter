// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package dependency

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

// TestTransitiveResolverResolve covers the DepsResolver adapter.
func TestTransitiveResolverResolve(t *testing.T) {
	t.Parallel()

	var resolver DepsResolver = transitiveResolver{}

	got, err := resolver.Resolve([]string{consts.Go}, map[string][]string{consts.Go: {}})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != consts.IndexZero {
		t.Fatalf("got %#v, want empty", got)
	}
}
