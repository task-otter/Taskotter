// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package faults

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

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
