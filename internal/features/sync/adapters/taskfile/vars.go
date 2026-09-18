// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"errors"
)

var (
	errNoModuleVars            = errors.New("module Taskfile has no vars")
	errInvalidYAMLNodePosition = errors.New("invalid YAML node position")
)
