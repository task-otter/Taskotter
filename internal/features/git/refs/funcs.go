// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package refs

import (
	"fmt"
	"strings"
)

// Validate rejects empty, option-like, oversized, or malformed references.
func Validate(ref string) error {
	ref = strings.TrimSpace(ref)

	if ref == "" {
		return fmt.Errorf("%w: must not be empty", ErrInvalidGitRef)
	}

	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("%w: must not start with '-'", ErrInvalidGitRef)
	}

	if len(ref) > maxRefLength {
		return fmt.Errorf("%w: exceeds maximum length", ErrInvalidGitRef)
	}

	if !refPattern.MatchString(ref) {
		return fmt.Errorf("%w: %q contains invalid characters", ErrInvalidGitRef, ref)
	}

	return nil
}
