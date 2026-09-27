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

const (
	fixtureVersionKey   = "version"
	fixtureVarsKey      = "vars"
	fixtureGoVersionKey = "GO_VERSION"
	fixtureVersion      = "3"
	fixtureGoVersion    = "1.26.5"
)

// TestMarshalAddsSingleDocumentStartAndTrailingNewline verifies output has a doc marker and trailing newline.
func TestMarshalAddsSingleDocumentStartAndTrailingNewline(t *testing.T) {
	t.Parallel()

	got, err := yamlfmt.Marshal(map[string]any{
		fixtureVersionKey: fixtureVersion,
		fixtureVarsKey: map[string]string{
			fixtureGoVersionKey: fixtureGoVersion,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "---\nvars:\n  " + fixtureGoVersionKey + ": " + fixtureGoVersion +
		"\nversion: \"" + fixtureVersion + "\"\n"

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
