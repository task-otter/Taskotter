// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package logging

import (
	"io"

	"github.com/rs/zerolog"
)

type (
	// Emitter is the logging surface used by sync orchestration.
	Emitter interface {
		Err() error
		Errorf(format string, args ...any)
		Noticef(format string, args ...any)
		Printf(format string, args ...any)
		Warningf(format string, args ...any)
	}

	// Logger emits GitHub Actions log commands.
	Logger struct {
		output zerolog.Logger
		sink   *failureSink
	}

	failureSink struct {
		destination io.Writer
		firstError  error
	}
)
