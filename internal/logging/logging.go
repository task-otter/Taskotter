// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

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
}

// New returns a logger writing to stdout.
func New() *Logger {
	return &Logger{out: os.Stdout}
}

// NewWithWriter returns a logger writing to w.
func NewWithWriter(w io.Writer) *Logger {
	return &Logger{out: w}
}

// Printf writes a plain log line.
func (l *Logger) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.out, format+"\n", args...)
}

// Group runs fn inside a GitHub Actions log group.
func (l *Logger) Group(name string, fn func()) {
	_, _ = fmt.Fprintf(l.out, "::group::%s\n", name)

	fn()

	_, _ = fmt.Fprintln(l.out, "::endgroup::")
}

// Noticef writes a GitHub Actions notice annotation.
func (l *Logger) Noticef(format string, args ...any) {
	_, _ = fmt.Fprintf(l.out, "::notice::"+format+"\n", args...)
}

// Warningf writes a GitHub Actions warning annotation.
func (l *Logger) Warningf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.out, "::warning::"+format+"\n", args...)
}

// Errorf writes a GitHub Actions error annotation.
func (l *Logger) Errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.out, "::error::"+format+"\n", args...)
}

// Redact replaces s with asterisks for safe logging.
func Redact(s string) string {
	if s == "" {
		return s
	}

	return strings.Repeat("*", len(s))
}
