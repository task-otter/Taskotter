// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package repo parses and validates "owner/name" GitHub repository coordinates.
package repo

import (
	"errors"
	"fmt"
	"strings"

	"github.com/task-otter/Taskotter/internal/consts"
)

var errInvalidRepository = errors.New("invalid repository")

// Parse splits owner/name repository coordinates.
func Parse(full string) (owner, name string, err error) {
	parts := strings.Split(full, "/")

	if len(parts) != consts.IndexTwo || parts[consts.IndexZero] == consts.Empty ||
		parts[consts.IndexOne] == consts.Empty {

		return consts.Empty, consts.Empty, fmt.Errorf("%w %q", errInvalidRepository, full)
	}

	return parts[consts.IndexZero], parts[consts.IndexOne], nil
}
