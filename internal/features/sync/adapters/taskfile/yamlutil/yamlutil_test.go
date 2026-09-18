// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlutil

import (
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

const (
	testOldKey       = "old"
	testValue        = "value"
	testUpdatedValue = "updated"
	testNewKey       = "new"
	testCloneValue   = "changed"
)

// TestNodeHelpersPreserveMappingSemantics verifies mapping mutations preserve keys.
func TestNodeHelpersPreserveMappingSemantics(t *testing.T) {
	t.Parallel()

	mapping := Mapping()
	AppendMappingPair(mapping, Scalar(testOldKey), Scalar(testValue))
	SetValue(mapping, testOldKey, Scalar(testUpdatedValue))
	SetValue(mapping, testNewKey, Scalar(testValue))

	if got := len(Keys(mapping)); got != mappingPairWidth {
		t.Fatalf("mapping key count = %d, want %d", got, mappingPairWidth)
	}

	DeleteKey(mapping, testOldKey)

	if got := len(Keys(mapping)); got != firstIndex {
		t.Fatalf("mapping key count after delete = %d, want %d", got, firstIndex)
	}
}

// TestCloneDeepCopiesContent verifies cloned nodes do not alias their source.
func TestCloneDeepCopiesContent(t *testing.T) {
	t.Parallel()

	original := Mapping()
	AppendMappingPair(original, Scalar("key"), Scalar(testValue))

	clone := Clone(original)

	clone.Content[firstIndex].Value = testCloneValue

	if original.Content[firstIndex].Value != testValue {
		t.Fatalf("clone mutated original: %q", original.Content[firstIndex].Value)
	}
}

// TestScalarAndSequenceKinds verifies YAML helper node kinds.
func TestScalarAndSequenceKinds(t *testing.T) {
	t.Parallel()

	if Scalar(testValue).Kind != yaml.ScalarNode {
		t.Fatal("scalar has wrong kind")
	}

	if Sequence().Kind != yaml.SequenceNode {
		t.Fatal("sequence has wrong kind")
	}
}

func TestSortedKeys(t *testing.T) {
	t.Parallel()

	got := SortedKeys(map[string]struct{}{"z": {}, "a": {}})

	if len(got) != 2 || got[0] != "a" || got[1] != "z" {
		t.Fatalf("SortedKeys() = %#v", got)
	}
}
