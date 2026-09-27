// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive

import (
	"path/filepath"
)

//nolint:gochecknoglobals // seam so tests can reach the filepath.Abs failure branches
var absPath = filepath.Abs
