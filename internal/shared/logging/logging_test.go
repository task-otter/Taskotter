// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package logging_test

import (
	"bytes"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

// TestLoggerWritesGitHubActionsCommands verifies log levels emit GitHub Actions workflow commands.
func TestLoggerWritesGitHubActionsCommands(t *testing.T) {
	t.Parallel()

	got := capturedLogOutput()

	want := consts.Empty +
		"plain line\n" +
		"::notice::notice 1\n" +
		"::warning::warning 2\n" +
		"::error::error 3\n" +
		"::group::sync\n" +
		"inside\n" +
		"::endgroup::\n"

	if got != want {
		t.Fatalf("log output = %q, want %q", got, want)
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

// TestNew verifies New returns a non-nil logger.
func TestNew(t *testing.T) {
	t.Parallel()

	if logging.New() == nil {
		t.Fatal("New() returned nil")
	}
}

// TestRedact verifies secret values are masked while empty input stays empty.
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
