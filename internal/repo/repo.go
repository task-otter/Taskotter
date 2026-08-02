// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package repo

import (
	"errors"
	"fmt"
	"strings"
)

var errInvalidRepository = errors.New("invalid repository")

// Parse splits owner/name repository coordinates.
func Parse(full string) (string, string, error) {
	parts := strings.Split(full, "/")

	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w %q", errInvalidRepository, full)
	}

	return parts[0], parts[1], nil
}
