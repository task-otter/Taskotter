// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package logging

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// New returns a logger writing to stdout.
func New() *Logger {
	return NewWithWriter(os.Stdout)
}

// NewWithWriter returns a logger writing to w.
func NewWithWriter(writer io.Writer) *Logger {
	var writeErr error

	return &Logger{
		write: func(text string) {
			if writeErr != nil {
				return
			}

			err := iox.WriteStringFull(writer, text)
			if err != nil {
				writeErr = fmt.Errorf("write log output: %w", err)
			}
		},
		err: func() error {
			return writeErr
		},
	}
}

// Err returns the first write error encountered by the logger, if any.
func (logger *Logger) Err() error {
	if logger.err == nil {
		return nil
	}

	if logger.cachedErr == nil {
		err := logger.err()
		if err != nil {
			logger.cachedErr = fmt.Errorf("logger error: %w", err)
		}
	}

	return logger.cachedErr
}

// Errorf writes a GitHub Actions error annotation.
func (logger *Logger) Errorf(format string, args ...any) {
	logger.write(fmt.Sprintf("::error::"+format+"\n", args...))
}

// Group runs fn inside a GitHub Actions log group.
func (logger *Logger) Group(name string, fn func()) {
	logger.write(fmt.Sprintf("::group::%s\n", name))

	fn()

	logger.write("::endgroup::\n")
}

// Noticef writes a GitHub Actions notice annotation.
func (logger *Logger) Noticef(format string, args ...any) {
	logger.write(fmt.Sprintf("::notice::"+format+"\n", args...))
}

// Print writes a plain log line without formatting.
func (logger *Logger) Print(text string) {
	logger.write(text)
}

// Printf writes a plain log line.
func (logger *Logger) Printf(format string, args ...any) {
	logger.write(fmt.Sprintf(format+"\n", args...))
}

// Warningf writes a GitHub Actions warning annotation.
func (logger *Logger) Warningf(format string, args ...any) {
	logger.write(fmt.Sprintf("::warning::"+format+"\n", args...))
}

// Redact replaces s with asterisks for safe logging.
func Redact(s string) string {
	if s == "" {
		return s
	}

	return strings.Repeat("*", len(s))
}
