// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package safety

import (
	"testing"
)

// TestIsSafeTarPath verifies archive paths stay within the extraction root.
func TestIsSafeTarPath(t *testing.T) {
	tests := map[string]bool{
		"taskfiles/go/Taskfile.yml": true,
		"../outside":                false,
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
	if !Escapes("../outside") || Escapes("inside/file") {
		t.Fatal("unexpected extraction escape result")
	}
}
