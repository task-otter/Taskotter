// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package apply

import (
	"errors"
	"testing"
)

func TestApplyRejectsPlanWithoutInput(t *testing.T) {
	t.Parallel()

	err := Apply(&Plan{})
	if !errors.Is(err, ErrMissingInput) {
		t.Fatalf("Apply() error = %v, want %v", err, ErrMissingInput)
	}
}
