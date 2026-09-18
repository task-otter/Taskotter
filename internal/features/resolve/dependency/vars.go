// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package dependency

import (
	"errors"
)

var (
	errModuleNotDefined = errors.New("module is not defined in .deps.yml")

	_ DepsResolver = transitiveResolver{}
)
