// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package refs

import "testing"

func TestValidateGitRefs(t *testing.T) {
	t.Parallel()

	for _, ref := range []string{"main", "taskotter/sync-abc", " v1.2.3 "} {
		err := Validate(ref)
		if err != nil {
			t.Fatalf("Validate(%q): %v", ref, err)
		}
	}

	for _, ref := range []string{"", "-main", string(make([]byte, maxRefLength+1)), "bad ref"} {
		err := Validate(ref)
		if err == nil {
			t.Fatalf("Validate(%q) accepted", ref)
		}
	}
}
