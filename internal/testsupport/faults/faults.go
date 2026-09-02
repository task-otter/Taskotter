// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package faults provides fault-injecting I/O doubles used to exercise error paths in tests.
package faults

import (
	"errors"
	"fmt"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	// StubReader reports a fixed error, or ErrFault when Err is nil.
	StubReader struct {
		Err error
	}

	// StubWriter reports Count bytes written and Err, letting tests drive short,
	// negative, and failing write branches. A nil Err reports success.
	StubWriter struct {
		Err   error
		Count int
	}
)

const (
	errFmt = "faults: %w"
)

// ErrFault is the sentinel error reported when a double has no configured error.
var ErrFault = errors.New("injected fault")

// Read consumes nothing and always fails.
func (reader *StubReader) Read(data []byte) (int, error) {
	return len(data) * consts.IndexZero, fmt.Errorf(errFmt, faultOr(reader.Err))
}

// Write reports the configured count and error, ignoring data.
func (writer *StubWriter) Write(data []byte) (int, error) {
	if writer.Err == nil {
		return writer.Count + len(data)*consts.IndexZero, nil
	}

	return writer.Count, fmt.Errorf(errFmt, writer.Err)
}

func faultOr(err error) error {
	if err != nil {
		return err
	}

	return ErrFault
}
