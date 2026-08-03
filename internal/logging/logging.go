// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package logging provides GitHub Actions-aware logging and secret redaction.
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Logger emits GitHub Actions log commands.
type Logger struct {
	out io.Writer
	err error
}

// New returns a logger writing to stdout.
func New() *Logger {
	return &Logger{out: os.Stdout}
}

// NewWithWriter returns a logger writing to w.
func NewWithWriter(w io.Writer) *Logger {
	return &Logger{out: w}
}

// Err returns the first write error encountered by the logger, if any.
func (l *Logger) Err() error {
	return l.err
}

func (l *Logger) write(s string) {
	if l.err != nil {
		return
	}

	_, err := io.WriteString(l.out, s)
	if err != nil {
		l.err = fmt.Errorf("write log output: %w", err)
	}
}

// Printf writes a plain log line.
func (l *Logger) Printf(format string, args ...any) {
	l.write(fmt.Sprintf(format+"\n", args...))
}

// Group runs fn inside a GitHub Actions log group.
func (l *Logger) Group(name string, fn func()) {
	l.write(fmt.Sprintf("::group::%s\n", name))

	fn()

	l.write("::endgroup::\n")
}

// Noticef writes a GitHub Actions notice annotation.
func (l *Logger) Noticef(format string, args ...any) {
	l.write(fmt.Sprintf("::notice::"+format+"\n", args...))
}

// Warningf writes a GitHub Actions warning annotation.
func (l *Logger) Warningf(format string, args ...any) {
	l.write(fmt.Sprintf("::warning::"+format+"\n", args...))
}

// Errorf writes a GitHub Actions error annotation.
func (l *Logger) Errorf(format string, args ...any) {
	l.write(fmt.Sprintf("::error::"+format+"\n", args...))
}

// Redact replaces s with asterisks for safe logging.
func Redact(s string) string {
	if s == "" {
		return s
	}

	return strings.Repeat("*", len(s))
}
