package yamlfmt_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/yamlfmt"
	"gopkg.in/yaml.v3"
)

func TestMarshalAddsSingleDocumentStartAndTrailingNewline(t *testing.T) {
	t.Parallel()

	got, err := yamlfmt.Marshal(map[string]any{
		"version": "3",
		"vars": map[string]string{
			"GO_VERSION": "1.26.5",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "---\nvars:\n  GO_VERSION: 1.26.5\nversion: \"3\"\n"
	if string(got) != want {
		t.Fatalf("Marshal() = %q, want %q", got, want)
	}
}

func TestMarshalReportsUnsupportedValue(t *testing.T) {
	t.Parallel()

	_, err := yamlfmt.Marshal(&yaml.Node{
		Kind:        99,
		Style:       0,
		Tag:         "",
		Value:       "",
		Anchor:      "",
		Alias:       nil,
		Content:     nil,
		HeadComment: "",
		LineComment: "",
		FootComment: "",
		Line:        0,
		Column:      0,
	})
	if err == nil {
		t.Fatal("expected marshal error")
	}
}
