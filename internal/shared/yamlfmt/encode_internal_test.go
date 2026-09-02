// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
	yaml "go.yaml.in/yaml/v3"
)

const (
	wantEncodeErr = "expected encode error"
)

// TestEncodeAndCloseReportsWriterFailure verifies writer failures surface from encode or close.
func TestEncodeAndCloseReportsWriterFailure(t *testing.T) {
	t.Parallel()

	writer := &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault}

	err := encodeAndClose(yaml.NewEncoder(writer), map[string]string{"key": "value"})
	if err == nil {
		t.Fatal(wantEncodeErr)
	}
}
