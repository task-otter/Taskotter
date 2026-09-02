// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package iox_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
)

const (
	payload      = "abc"
	wantErrFmt   = "%s: expected error"
	unexpectFmt  = "%s: unexpected error: %v"
	outputsName  = "outputs.txt"
	multilineVal = "one\ntwo"
	stringVerb   = "%s"
	keyName      = "key"
	opWriteFull  = "WriteFull"
	opWriteStr   = "WriteStringFull"
	opCopy       = "CopyDiscard"
	opOutputs    = "WriteGitHubOutputs"
	opFormat     = "format write"
)

// TestDiscardAcceptsAnyValue verifies both the empty and non-empty branches are exercised.
func TestDiscardAcceptsAnyValue(t *testing.T) {
	t.Parallel()
	iox.Discard(consts.Empty)
	iox.Discard(payload)
	iox.Discard2(consts.Empty, payload)
}

// TestWriteFullWritesAllBytes verifies partial writes are retried until the data is consumed.
func TestWriteFullWritesAllBytes(t *testing.T) {
	t.Parallel()

	var sink strings.Builder

	err := iox.WriteFull(&sink, []byte(payload))
	if err != nil {
		t.Fatalf(unexpectFmt, opWriteFull, err)
	}

	if sink.String() != payload {
		t.Fatalf("WriteFull wrote %q, want %q", sink.String(), payload)
	}
}

// TestWriteFullRetriesPartialWrites verifies a one-byte-at-a-time writer still completes.
func TestWriteFullRetriesPartialWrites(t *testing.T) {
	t.Parallel()

	writer := &faults.StubWriter{Count: consts.IndexOne, Err: nil}

	err := iox.WriteFull(writer, []byte(payload))
	if err != nil {
		t.Fatalf(unexpectFmt, "WriteFull partial", err)
	}
}

// TestWriteFullReportsFailures verifies write errors and stalled writers are reported.
func TestWriteFullReportsFailures(t *testing.T) {
	t.Parallel()

	cases := []*faults.StubWriter{
		{Count: consts.IndexZero, Err: faults.ErrFault},
		{Count: consts.IndexZero, Err: nil},
	}

	for i := range cases {
		err := iox.WriteFull(cases[i], []byte(payload))
		if err == nil {
			t.Fatalf(wantErrFmt, opWriteFull)
		}
	}
}

// TestWriteStringFullReportsFailure verifies the string wrapper propagates write errors.
func TestWriteStringFullReportsFailure(t *testing.T) {
	t.Parallel()

	var sink strings.Builder

	err := iox.WriteStringFull(&sink, payload)
	if err != nil {
		t.Fatalf(unexpectFmt, opWriteStr, err)
	}

	err = iox.WriteStringFull(
		&faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault},
		payload,
	)
	if err == nil {
		t.Fatalf(wantErrFmt, opWriteStr)
	}
}

// TestCopyDiscardConsumesReader verifies successful and failing copies.
func TestCopyDiscardConsumesReader(t *testing.T) {
	t.Parallel()

	err := iox.CopyDiscard(strings.NewReader(payload))
	if err != nil {
		t.Fatalf(unexpectFmt, opCopy, err)
	}

	err = iox.CopyDiscard(&faults.StubReader{Err: nil})
	if err == nil {
		t.Fatalf(wantErrFmt, opCopy)
	}
}

// TestFprintfReportsFailures verifies write errors and negative counts are reported.
func TestFprintfReportsFailures(t *testing.T) {
	t.Parallel()
	assertFormatWrite(t, iox.Fprintf)
}

// TestFprintReportsFailures verifies write errors and negative counts are reported.
func TestFprintReportsFailures(t *testing.T) {
	t.Parallel()
	assertArgsWrite(t, iox.Fprint)
}

// TestFprintlnReportsFailures verifies write errors and negative counts are reported.
func TestFprintlnReportsFailures(t *testing.T) {
	t.Parallel()
	assertArgsWrite(t, iox.Fprintln)
}

// TestBestEffortWritersIgnoreFailures verifies the best-effort helpers swallow every outcome.
func TestBestEffortWritersIgnoreFailures(t *testing.T) {
	t.Parallel()

	writers := bestEffortWriters()

	for i := range writers {
		writer := writers[i]
		iox.FprintfBestEffortf(writer, stringVerb, payload)
		iox.FprintBestEffort(writer, payload)
		iox.FprintlnBestEffort(writer, payload)
	}
}

// TestWriteGitHubOutputsSkipsEmptyPath verifies an empty path is a no-op.
func TestWriteGitHubOutputsSkipsEmptyPath(t *testing.T) {
	t.Parallel()

	err := iox.WriteGitHubOutputs(consts.Empty, map[string]string{keyName: payload})
	if err != nil {
		t.Fatalf(unexpectFmt, opOutputs, err)
	}
}

// TestWriteGitHubOutputsWritesFile verifies single-line and multi-line values are encoded.
func TestWriteGitHubOutputsWritesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), outputsName)

	err := iox.WriteGitHubOutputs(path, map[string]string{"plain": payload, "multi": multilineVal})
	if err != nil {
		t.Fatalf(unexpectFmt, opOutputs, err)
	}

	assertOutputsFile(t, path)
}

// TestWriteGitHubOutputsReportsWriteFailure verifies an unwritable path returns an error.
func TestWriteGitHubOutputsReportsWriteFailure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), outputsName, "nested")

	err := iox.WriteGitHubOutputs(path, map[string]string{keyName: payload})
	if err == nil {
		t.Fatalf(wantErrFmt, opOutputs)
	}
}

func assertArgsWrite(t *testing.T, write func(writer io.Writer, args ...any) error) {
	t.Helper()

	assertFormatWrite(t, func(writer io.Writer, format string, args ...any) error {
		return write(writer, append([]any{format}, args...)...)
	})
}

func assertFormatWrite(
	t *testing.T,
	write func(writer io.Writer, format string, args ...any) error,
) {
	t.Helper()

	var sink strings.Builder

	err := write(&sink, stringVerb, payload)
	if err != nil {
		t.Fatalf(unexpectFmt, opFormat, err)
	}

	writers := failingWriters()

	for i := range writers {
		err = write(writers[i], stringVerb, payload)
		if err == nil {
			t.Fatalf(wantErrFmt, opFormat)
		}
	}
}

func assertOutputsFile(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // path is a file this test just created
	if err != nil {
		t.Fatalf(unexpectFmt, "read outputs", err)
	}

	text := string(data)

	if !strings.Contains(text, "plain="+payload) || !strings.Contains(text, "multi<<EOF") {
		t.Fatalf("outputs = %q", text)
	}
}

func bestEffortWriters() []*faults.StubWriter {
	return append(failingWriters(), &faults.StubWriter{Count: len(payload), Err: nil})
}

func failingWriters() []*faults.StubWriter {
	return []*faults.StubWriter{
		{Count: consts.IndexZero, Err: faults.ErrFault},
		{Count: -consts.IndexOne, Err: nil},
	}
}
