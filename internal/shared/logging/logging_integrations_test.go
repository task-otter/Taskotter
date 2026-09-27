// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package logging_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/logging"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
)

const (
	errLogOutputFmt  = "log output = %q, want %q"
	errUnexpectedFmt = "Err() = %v, want %v"
)

// TestLoggerWritesGitHubActionsCommands verifies the behavior covered by this test.
func TestLoggerWritesGitHubActionsCommands(t *testing.T) {
	t.Parallel()

	got := capturedLogOutput()

	want := consts.Empty +
		"{\"level\":\"info\",\"message\":\"plain line\"}\n" +
		"::notice::notice 1\n" +
		"::warning::warning 2\n" +
		"::error::error 3\n" +
		"::group::sync\n" +
		"{\"level\":\"info\",\"message\":\"inside\\n\"}\n" +
		"::endgroup::\n"

	if got != want {
		t.Fatalf(errLogOutputFmt, got, want)
	}
}

// TestLoggerEscapesJSONMessage verifies the behavior covered by this test.
func TestLoggerEscapesJSONMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	log := logging.NewWithWriter(&buf)
	log.Print("quoted \"value\"\nnext")

	want := "{\"level\":\"info\",\"message\":\"quoted \\\"value\\\"\\nnext\"}\n"

	if buf.String() != want {
		t.Fatalf(errLogOutputFmt, buf.String(), want)
	}
}

// TestNew verifies the behavior covered by this test.
func TestNew(t *testing.T) {
	t.Parallel()

	if logging.New() == nil {
		t.Fatal("New() returned nil")
	}
}

// TestLoggerRecordsFirstWriteError verifies the behavior covered by this test.
func TestLoggerRecordsFirstWriteError(t *testing.T) {
	t.Parallel()

	writer := &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault}
	log := logging.NewWithWriter(writer)

	log.Print("first\n")

	first := log.Err()
	if first == nil {
		t.Fatal("Err() = nil, want write error")
	}

	log.Print("second\n")

	if !errors.Is(log.Err(), first) {
		t.Fatal("Err() changed after the first failure")
	}
}

// TestLoggerRecordsShortWrite verifies the behavior covered by this test.
func TestLoggerRecordsShortWrite(t *testing.T) {
	t.Parallel()

	writer := &faults.StubWriter{Count: consts.IndexZero}
	log := logging.NewWithWriter(writer)

	log.Print("incomplete")

	if !errors.Is(log.Err(), io.ErrShortWrite) {
		t.Fatalf(errUnexpectedFmt, log.Err(), io.ErrShortWrite)
	}
}

// TestLoggerRecordsCommandWriteError verifies the behavior covered by this test.
func TestLoggerRecordsCommandWriteError(t *testing.T) {
	t.Parallel()

	writer := &faults.StubWriter{Count: consts.IndexZero, Err: faults.ErrFault}
	log := logging.NewWithWriter(writer)

	log.Noticef("failed command")

	if !errors.Is(log.Err(), faults.ErrFault) {
		t.Fatalf(errUnexpectedFmt, log.Err(), faults.ErrFault)
	}
}

// TestLoggerErrIsNilWhenWritesSucceed verifies the behavior covered by this test.
func TestLoggerErrIsNilWhenWritesSucceed(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	log := logging.NewWithWriter(&buf)
	log.Print("ok\n")

	if log.Err() != nil {
		t.Fatalf("Err() = %v, want nil", log.Err())
	}
}

// TestRedact verifies the behavior covered by this test.
func TestRedact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{consts.Empty, consts.Empty},
		{"token", "*****"},
	}

	for i := range cases {
		testCase := &cases[i]
		got := logging.Redact(testCase.input)

		if got != testCase.want {
			t.Fatalf("Redact(%q) = %q, want %q", testCase.input, got, testCase.want)
		}
	}
}

func capturedLogOutput() string {
	var buf bytes.Buffer

	log := logging.NewWithWriter(&buf)

	log.Printf("plain %s", "line")
	log.Noticef("notice %d", consts.IndexOne)
	log.Warningf("warning %d", consts.IndexTwo)
	log.Errorf("error %d", consts.IndexThree)
	log.Group("sync", func() {
		log.Print("inside\n")
	})

	return buf.String()
}

// TestLoggerErrNilWriteFunc verifies the behavior covered by this test.
func TestLoggerErrNilWriteFunc(t *testing.T) {
	t.Parallel()

	logger := &logging.Logger{}

	if logger.Err() != nil {
		t.Fatalf("Err = %v", logger.Err())
	}

	logger.Noticef("ignored")
}
