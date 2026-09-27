// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"io"
	"os"
)

//nolint:gochecknoglobals // seams so tests can drive main without exiting the test process
var (
	exitFunc           = os.Exit
	stdout   io.Writer = os.Stdout
	stderr   io.Writer = os.Stderr
	wireRun  wireRunFn = defaultWireRun
)
