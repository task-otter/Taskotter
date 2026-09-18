// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package hash

import (
	"errors"
)

// ErrInvalidPrefixLength identifies an invalid branch-hash prefix size.
var ErrInvalidPrefixLength = errors.New("invalid hash prefix length")
