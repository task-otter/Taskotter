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
	testPairCount    = 2
	testSingleCount  = 1
	secondPairIndex  = 1
)

// TestNodeHelpersPreserveMappingSemantics verifies mapping mutations preserve keys.
func TestNodeHelpersPreserveMappingSemantics(t *testing.T) {
	mapping := Mapping()
	AppendMappingPair(mapping, Scalar(testOldKey), Scalar(testValue))
	SetValue(mapping, testOldKey, Scalar(testUpdatedValue))
	SetValue(mapping, testNewKey, Scalar(testValue))

	if got := len(Keys(mapping)); got != testPairCount {
		t.Fatalf("mapping key count = %d, want %d", got, testPairCount)
	}

	DeleteKey(mapping, testOldKey)

	if got := len(Keys(mapping)); got != testSingleCount {
		t.Fatalf("mapping key count after delete = %d, want %d", got, testSingleCount)
	}
}

// TestCloneDeepCopiesContent verifies cloned nodes do not alias their source.
func TestCloneDeepCopiesContent(t *testing.T) {
	original := Mapping()
	AppendMappingPair(original, Scalar("key"), Scalar(testValue))

	clone := Clone(original)

	clone.Content[secondPairIndex].Value = testCloneValue

	if original.Content[secondPairIndex].Value != testValue {
		t.Fatalf("clone mutated original: %q", original.Content[secondPairIndex].Value)
	}
}

// TestScalarAndSequenceKinds verifies YAML helper node kinds.
func TestScalarAndSequenceKinds(t *testing.T) {
	if Scalar(testValue).Kind != yaml.ScalarNode {
		t.Fatal("scalar has wrong kind")
	}

	if Sequence().Kind != yaml.SequenceNode {
		t.Fatal("sequence has wrong kind")
	}
}
