// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package faults_test

import (
	"errors"
	"io"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
)

const (
	payload  = "abc"
	countFmt = "count = %d, want %d"
)

var errCustom = errors.New("custom")

// TestStubReaderReportsError verifies the reader fails with the configured and default errors.
func TestStubReaderReportsError(t *testing.T) {
	t.Parallel()

	cases := []error{nil, errCustom}

	for i := range cases {
		assertStubRead(t, cases[i])
	}
}

// TestStubWriterReportsError verifies a configured error surfaces with the configured count.
func TestStubWriterReportsError(t *testing.T) {
	t.Parallel()
	assertStubWriteFails(t, errCustom)
}

// TestStubWriterReportsCount verifies short and negative counts are reported without an error.
func TestStubWriterReportsCount(t *testing.T) {
	t.Parallel()

	cases := []int{consts.IndexZero, consts.IndexOne, -consts.IndexOne}

	for i := range cases {
		assertStubWriteCount(t, cases[i])
	}
}

func assertStubRead(t *testing.T, configured error) {
	t.Helper()

	count, err := io.Reader(&faults.StubReader{Err: configured}).Read(make([]byte, len(payload)))

	if count != consts.IndexZero {
		t.Fatalf(countFmt, count, consts.IndexZero)
	}

	assertFaulted(t, err, configured)
}

func assertStubWriteFails(t *testing.T, configured error) {
	t.Helper()

	writer := &faults.StubWriter{Count: consts.IndexZero, Err: configured}

	count, err := io.Writer(writer).Write([]byte(payload))

	if count != consts.IndexZero {
		t.Fatalf(countFmt, count, consts.IndexZero)
	}

	assertFaulted(t, err, configured)
}

func assertStubWriteCount(t *testing.T, want int) {
	t.Helper()

	writer := &faults.StubWriter{Count: want, Err: nil}

	count, err := io.Writer(writer).Write([]byte(payload))
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if count != want {
		t.Fatalf(countFmt, count, want)
	}
}

func assertFaulted(t *testing.T, err, configured error) {
	t.Helper()

	want := configured
	if want == nil {
		want = faults.ErrFault
	}

	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
