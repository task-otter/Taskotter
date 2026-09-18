// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package refs

import (
	"errors"
	"regexp"
)

var (
	// ErrInvalidGitRef identifies unsafe Git references.
	ErrInvalidGitRef = errors.New("invalid git ref")
	refPattern       = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
)
