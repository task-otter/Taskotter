// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlutil

import (
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

// TestNodeHelpersPreserveMappingSemantics verifies mapping mutations preserve keys.
func TestNodeHelpersPreserveMappingSemantics(t *testing.T) {
	mapping := Mapping()
	AppendMappingPair(mapping, Scalar("old"), Scalar("value"))
	SetValue(mapping, "old", Scalar("updated"))
	SetValue(mapping, "new", Scalar("value"))

	if got := len(Keys(mapping)); got != 2 {
		t.Fatalf("mapping key count = %d, want 2", got)
	}

	DeleteKey(mapping, "old")

	if got := len(Keys(mapping)); got != 1 {
		t.Fatalf("mapping key count after delete = %d, want 1", got)
	}
}

// TestCloneDeepCopiesContent verifies cloned nodes do not alias their source.
func TestCloneDeepCopiesContent(t *testing.T) {
	original := Mapping()
	AppendMappingPair(original, Scalar("key"), Scalar("value"))

	clone := Clone(original)

	clone.Content[1].Value = "changed"

	if original.Content[1].Value != "value" {
		t.Fatalf("clone mutated original: %q", original.Content[1].Value)
	}
}

// TestScalarAndSequenceKinds verifies YAML helper node kinds.
func TestScalarAndSequenceKinds(t *testing.T) {
	if Scalar("value").Kind != yaml.ScalarNode {
		t.Fatal("scalar has wrong kind")
	}

	if Sequence().Kind != yaml.SequenceNode {
		t.Fatal("sequence has wrong kind")
	}
}
