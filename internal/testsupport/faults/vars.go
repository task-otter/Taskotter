// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package faults

import (
	"errors"
)

// ErrFault is the sentinel error reported when a double has no configured error.
var ErrFault = errors.New("injected fault")
