// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package logging

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// New creates a logger that writes to the default output.
func New() *Logger {
	return NewWithWriter(os.Stdout)
}

// NewWithWriter creates a logger that writes to the supplied writer.
func NewWithWriter(writer io.Writer) *Logger {
	sink := &failureSink{destination: writer}

	return &Logger{
		output: zerolog.New(sink),
		sink:   sink,
	}
}

// Err returns the first error recorded by the logger.
func (logger *Logger) Err() error {
	if logger.sink == nil {
		return nil
	}

	return logger.sink.Err()
}

// Errorf writes the corresponding formatted message or logging group.
func (logger *Logger) Errorf(format string, args ...any) {
	logger.writeCommand(fmt.Sprintf("::error::"+format+"\n", args...))
}

// Group writes the corresponding formatted message or logging group.
func (logger *Logger) Group(name string, fn func()) {
	logger.writeCommand(fmt.Sprintf("::group::%s\n", name))

	fn()

	logger.writeCommand("::endgroup::\n")
}

// Noticef writes the corresponding formatted message or logging group.
func (logger *Logger) Noticef(format string, args ...any) {
	logger.writeCommand(fmt.Sprintf("::notice::"+format+"\n", args...))
}

// Print writes the corresponding formatted message or logging group.
func (logger *Logger) Print(text string) {
	logger.output.Info().Msg(text)
}

// Printf writes the corresponding formatted message or logging group.
func (logger *Logger) Printf(format string, args ...any) {
	logger.output.Info().Msgf(format, args...)
}

// Warningf writes the corresponding formatted message or logging group.
func (logger *Logger) Warningf(format string, args ...any) {
	logger.writeCommand(fmt.Sprintf("::warning::"+format+"\n", args...))
}

func (logger *Logger) writeCommand(command string) {
	if logger.sink == nil {
		return
	}

	written, err := logger.sink.Write([]byte(command))
	iox.Discard2(written, err)
}

func (sink *failureSink) Write(data []byte) (int, error) {
	if sink.firstError != nil {
		return consts.IndexZero, sink.firstError
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

func (sink *failureSink) Err() error {
	return sink.firstError
}

// Redact removes sensitive values from log text.
func Redact(s string) string {
	if s == "" {
		return s
	}

	return strings.Repeat("*", len(s))
}
