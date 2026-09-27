// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package logging

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// New returns a logger writing to stdout.
func New() *Logger {
	return NewWithWriter(os.Stdout)
}

// NewWithWriter returns a logger writing to w.
func NewWithWriter(writer io.Writer) *Logger {
	sink := &failureSink{destination: writer}

	return &Logger{
		output: zerolog.New(sink),
		sink:   sink,
	}
}

// Err returns the first write error encountered by the logger, if any.
func (logger *Logger) Err() error {
	if logger.sink == nil {
		return nil
	}

	return logger.sink.firstError
}

// Errorf writes a GitHub Actions error annotation.
func (logger *Logger) Errorf(format string, args ...any) {
	logger.writeCommand(fmt.Sprintf("::error::"+format+"\n", args...))
}

// Group runs fn inside a GitHub Actions log group.
func (logger *Logger) Group(name string, fn func()) {
	logger.writeCommand(fmt.Sprintf("::group::%s\n", name))

	fn()

	logger.writeCommand("::endgroup::\n")
}

// Noticef writes a GitHub Actions notice annotation.
func (logger *Logger) Noticef(format string, args ...any) {
	logger.writeCommand(fmt.Sprintf("::notice::"+format+"\n", args...))
}

// Print writes an info-level JSON log event.
func (logger *Logger) Print(text string) {
	logger.output.Info().Msg(text)
}

// Printf writes a formatted info-level JSON log event.
func (logger *Logger) Printf(format string, args ...any) {
	logger.output.Info().Msgf(format, args...)
}

// Warningf writes a GitHub Actions warning annotation.
func (logger *Logger) Warningf(format string, args ...any) {
	logger.writeCommand(fmt.Sprintf("::warning::"+format+"\n", args...))
}

func (logger *Logger) writeCommand(command string) {
	if logger.sink == nil {
		return
	}

	_, err := logger.sink.Write([]byte(command))
	if err != nil {
		return
	}
}

func (sink *failureSink) Write(data []byte) (int, error) {
	if sink.firstError != nil {
		return 0, sink.firstError
	}

	written, err := sink.destination.Write(data)

	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}

	if err != nil {
		sink.firstError = fmt.Errorf("write log output: %w", err)
	}

	return written, err
}

// Redact replaces s with asterisks for safe logging.
func Redact(s string) string {
	if s == "" {
		return s
	}

	return strings.Repeat("*", len(s))
}
