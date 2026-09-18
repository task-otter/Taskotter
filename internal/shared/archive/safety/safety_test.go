// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package safety

import (
	"testing"
)

const outsidePath = "../outside"

// TestIsSafeTarPath verifies archive paths stay within the extraction root.
func TestIsSafeTarPath(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"taskfiles/go/Taskfile.yml": true,
		outsidePath:                 false,
		"/absolute":                 false,
		"windows\\escape":           false,
	}

	for path, want := range tests {
		if got := IsSafeTarPath(path); got != want {
			t.Errorf("IsSafeTarPath(%q) = %t, want %t", path, got, want)
		}
	}
}

// TestEscapes verifies traversal paths are detected.
func TestEscapes(t *testing.T) {
	t.Parallel()

	if !Escapes(outsidePath) || Escapes("inside/file") {
		t.Fatal("unexpected extraction escape result")
	}
}
