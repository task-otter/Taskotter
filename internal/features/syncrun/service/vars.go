// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
)

var (
	errUnrelatedChanges          = errors.New("unrelated uncommitted changes detected in workspace")
	errOrchestratorNotConfigured = errors.New("orchestrator is not configured")
)
