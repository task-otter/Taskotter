// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package logging_test

import (
	"io"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

// BenchmarkLoggerPrint measures an unformatted zerolog event.
func BenchmarkLoggerPrint(b *testing.B) {
	logger := logging.NewWithWriter(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		logger.Print("sync complete")
	}
}

// BenchmarkLoggerPrintf measures a formatted zerolog event.
func BenchmarkLoggerPrintf(b *testing.B) {
	logger := logging.NewWithWriter(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		logger.Printf("files changed: %d", consts.IndexThree)
	}
}
