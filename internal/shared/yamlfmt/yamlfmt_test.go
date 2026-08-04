// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

// TestMarshalAddsSingleDocumentStartAndTrailingNewline verifies output has a doc marker and trailing newline.
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

// TestMarshalReportsUnsupportedValue verifies an unsupported YAML node kind returns an error.
func TestMarshalReportsUnsupportedValue(t *testing.T) {
	t.Parallel()

	got, err := yamlfmt.Marshal(unsupportedYAMLNode())
	iox.Discard(got)

	if err == nil {
		t.Fatal("expected marshal error")
	}
}

func unsupportedYAMLNode() *yaml.Node {
	return &yaml.Node{
		Kind:        consts.Index99,
		Style:       consts.IndexZero,
		Tag:         consts.Empty,
		Value:       consts.Empty,
		Anchor:      consts.Empty,
		Alias:       nil,
		Content:     nil,
		HeadComment: consts.Empty,
		LineComment: consts.Empty,
		FootComment: consts.Empty,
		Line:        consts.IndexZero,
		Column:      consts.IndexZero,
	}
}
