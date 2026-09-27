// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package root

import "testing"

func TestNewOpsProvidesRootOperations(t *testing.T) {
	t.Parallel()

	ops := NewOps()

	if len(ops.NewRootTemplate()) == 0 {
		t.Fatal("NewRootTemplate() returned empty content")
	}
}

func TestGenerateReturnsRootContent(t *testing.T) {
	t.Parallel()

	result, err := Generate(Input{}, NewOps())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("Generate() returned empty content")
	}
}
