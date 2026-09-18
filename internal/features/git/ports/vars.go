// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"errors"
)

var errBranchNotOwned = errors.New("branch exists but is not owned by TaskOtter")
