// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil

import (
	"errors"
	"path/filepath"
	"regexp"
)

var (
	windowsAbsPath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	taskNameRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

	errPathComponentNotExist = errors.New("path component does not exist")

	//nolint:gochecknoglobals // seam so tests can reach the filepath.Abs failure branches
	absPath = filepath.Abs
)
