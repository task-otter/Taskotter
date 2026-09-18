// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"time"
)

const (
	runTimeout        = 15 * time.Minute
	exitSuccess       = 0
	exitError         = 1
	errWireOrchFmt    = "wire orchestrator: %w"
	errCreatePRClient = "create GitHub PR client: %w"
)
