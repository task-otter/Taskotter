// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
)

var (
	errDepsMissingModule     = errors.New(".deps.yml references missing module")
	errDepsMissingDependency = errors.New(".deps.yml references missing dependency")
)
