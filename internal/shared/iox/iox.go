// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package iox provides dogsled-safe I/O helpers.
package iox

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	errFmtWrappedInt = "%w: %d"
	opFprint         = "fprint"
	opFprintln       = "fprintln"
	githubOutputPerm = 0o600
)

var (
	errShortWrite           = errors.New("short write")
	errInvalidCopyCount     = errors.New("invalid copy count")
	errInvalidFprintfCount  = errors.New("invalid fprintf count")
	errInvalidFprintCount   = errors.New("invalid fprint count")
	errInvalidFprintlnCount = errors.New("invalid fprintln count")
	errBuilderShortWrite    = errors.New("builder short write")
	errInvalidBuilderPrintf = errors.New("invalid builder printf count")
)

// Discard consumes v so callers can avoid blank identifiers under strict linters.
func Discard[T any](v T) {
	if fmt.Sprintf("%v", v) == "" {
		return
	}
}

// Discard2 consumes two values.
func Discard2[T, U any](first T, second U) {
	Discard(first)
	Discard(second)
}

func validateByteCount(count int, errInvalid error) error {
	if count < consts.IndexZero {
		return fmt.Errorf(errFmtWrappedInt, errInvalid, count)
	}

	return nil
}

func ignoreWriteResult(byteCount int, err error) {
	if err != nil {
		return
	}

	if byteCount < consts.IndexZero {
		return
	}
}

func formatWriteInvalid(operation string) error {
	if operation == opFprint {
		return errInvalidFprintCount
	}

	return errInvalidFprintlnCount
}

func finishStdFormatWrite(byteCount int, writeErr error, operation string) error {
	if writeErr != nil {
		return fmt.Errorf("%s: %w", operation, writeErr)
	}

	err := validateByteCount(byteCount, formatWriteInvalid(operation))
	if err != nil {
		return fmt.Errorf("%s count: %w", operation, err)
	}

	return nil
}

// WriteStringFull writes all of s to w or returns an error.
func WriteStringFull(w io.Writer, s string) error {
	err := WriteFull(w, []byte(s))
	if err != nil {
		return fmt.Errorf("write string full: %w", err)
	}

	return nil
}

// WriteFull writes all of data to w or returns an error.
func WriteFull(w io.Writer, data []byte) error {
	for len(data) > consts.IndexZero {
		written, err := w.Write(data)
		if err != nil {
			return fmt.Errorf("write: %w", err)
		}

		if written <= consts.IndexZero {
			return fmt.Errorf("%w: wrote %d bytes", errShortWrite, written)
		}

		data = data[written:]
	}

	return nil
}

// CopyDiscard copies r to [io.Discard] and returns any error.
func CopyDiscard(r io.Reader) error {
	byteCount, err := io.Copy(io.Discard, r)
	if err != nil {
		return fmt.Errorf("copy to discard: %w", err)
	}

	err = validateByteCount(int(byteCount), errInvalidCopyCount)
	if err != nil {
		return fmt.Errorf("copy discard count: %w", err)
	}

	return nil
}

// Fprintf formats and writes to w.
func Fprintf(w io.Writer, format string, args ...any) error {
	byteCount, err := fmt.Fprintf(w, format, args...)
	if err != nil {
		return fmt.Errorf("fprintf: %w", err)
	}

	err = validateByteCount(byteCount, errInvalidFprintfCount)
	if err != nil {
		return fmt.Errorf("fprintf count: %w", err)
	}

	return nil
}

// Fprint writes args to w.
func Fprint(w io.Writer, args ...any) error {
	byteCount, err := fmt.Fprint(w, args...)

	finishErr := finishStdFormatWrite(byteCount, err, opFprint)
	if finishErr != nil {
		return fmt.Errorf("fprint output: %w", finishErr)
	}

	return nil
}

// Fprintln writes args followed by a newline to w.
func Fprintln(w io.Writer, args ...any) error {
	byteCount, err := fmt.Fprintln(w, args...)

	finishErr := finishStdFormatWrite(byteCount, err, opFprintln)
	if finishErr != nil {
		return fmt.Errorf("fprintln output: %w", finishErr)
	}

	return nil
}

// FprintfBestEffortf formats and writes to w, ignoring errors.
func FprintfBestEffortf(w io.Writer, format string, args ...any) {
	byteCount, err := fmt.Fprintf(w, format, args...)
	ignoreWriteResult(byteCount, err)
}

// FprintBestEffort writes args to w, ignoring errors.
func FprintBestEffort(w io.Writer, args ...any) {
	byteCount, err := fmt.Fprint(w, args...)
	ignoreWriteResult(byteCount, err)
}

// FprintlnBestEffort writes args followed by a newline to w, ignoring errors.
func FprintlnBestEffort(w io.Writer, args ...any) {
	byteCount, err := fmt.Fprintln(w, args...)
	ignoreWriteResult(byteCount, err)
}

// BuilderWriteString writes text to b.
func BuilderWriteString(b *strings.Builder, text string) error {
	written, err := b.WriteString(text)
	if err != nil {
		return fmt.Errorf("builder write string: %w", err)
	}

	if written != len(text) {
		return fmt.Errorf("%w: wrote %d of %d", errBuilderShortWrite, written, len(text))
	}

	return nil
}

// BuilderPrintf formats and writes to b.
func BuilderPrintf(b *strings.Builder, format string, args ...any) error {
	byteCount, err := fmt.Fprintf(b, format, args...)
	if err != nil {
		return fmt.Errorf("builder printf: %w", err)
	}

	err = validateByteCount(byteCount, errInvalidBuilderPrintf)
	if err != nil {
		return fmt.Errorf("builder printf count: %w", err)
	}

	return nil
}

// WriteGitHubOutputs writes GitHub Actions step outputs to path.
func WriteGitHubOutputs(path string, values map[string]string) error {
	if path == consts.Empty {
		return nil
	}

	payload, err := buildGitHubOutputs(values)
	if err != nil {
		return fmt.Errorf("build GitHub Actions outputs: %w", err)
	}

	err = os.WriteFile(path, payload, githubOutputPerm)
	if err != nil {
		return fmt.Errorf("write GitHub Actions outputs: %w", err)
	}

	return nil
}

func buildGitHubOutputs(values map[string]string) ([]byte, error) {
	var output strings.Builder

	for key := range values {
		writeErr := BuilderWriteString(&output, formatGitHubOutputLine(key, values[key]))
		if writeErr != nil {
			return nil, fmt.Errorf("write output buffer: %w", writeErr)
		}
	}

	return []byte(output.String()), nil
}

func formatGitHubOutputLine(key, value string) string {
	if strings.Contains(value, consts.Newline) {
		return key + "<<EOF\n" + value + "\nEOF\n"
	}

	return key + "=" + value + consts.Newline
}
