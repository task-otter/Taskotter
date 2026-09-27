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
	//nolint:reusability // Logger intentionally owns its output and failure sink.
	Logger struct {
		output zerolog.Logger
		sink   logWriter
	}

	logWriter interface {
		io.Writer
		Err() error
	}

	//nolint:reusability // The sink is a private adapter for zerolog's writer contract.
	failureSink struct {
		destination io.Writer
		firstError  error
	}
)
