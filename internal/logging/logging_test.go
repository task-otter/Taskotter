package logging_test

import (
	"bytes"
	"testing"

	"github.com/task-otter/Taskotter/internal/logging"
)

func TestLoggerWritesGitHubActionsCommands(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	log := logging.NewWithWriter(&buf)

	log.Printf("plain %s", "line")
	log.Noticef("notice %d", 1)
	log.Warningf("warning %d", 2)
	log.Errorf("error %d", 3)
	log.Group("sync", func() {
		log.Printf("inside")
	})

	want := "" +
		"plain line\n" +
		"::notice::notice 1\n" +
		"::warning::warning 2\n" +
		"::error::error 3\n" +
		"::group::sync\n" +
		"inside\n" +
		"::endgroup::\n"
	if buf.String() != want {
		t.Fatalf("log output = %q, want %q", buf.String(), want)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()

	if logging.New() == nil {
		t.Fatal("New() returned nil")
	}
}

func TestRedact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"token", "*****"},
	}

	for _, testCase := range cases {
		got := logging.Redact(testCase.input)
		if got != testCase.want {
			t.Fatalf("Redact(%q) = %q, want %q", testCase.input, got, testCase.want)
		}
	}
}
