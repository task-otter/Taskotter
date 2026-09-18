// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package env

import "testing"

func TestInputAndToken(t *testing.T) {
	t.Setenv("INPUT_GITHUB-TOKEN", " hyphen-token ")
	t.Setenv("INPUT_GITHUB_TOKEN", "underscore-token")

	if got := Input("github-token"); got != "hyphen-token" {
		t.Fatalf("Input() = %q", got)
	}

	t.Setenv("INPUT_TASKS", " tasks ")

	if got := Input("tasks"); got != "tasks" {
		t.Fatalf("Input() = %q", got)
	}

	t.Setenv("INPUT_GITHUB-TOKEN", "")
	t.Setenv("INPUT_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", " fallback ")

	if got := Token(); got != "fallback" {
		t.Fatalf("Token() = %q", got)
	}
}

func TestInputMissing(t *testing.T) {
	t.Setenv("INPUT_MISSING", "")

	if got := Input("missing"); got != emptyValue {
		t.Fatalf("Input() = %q", got)
	}
}

func TestTokenUsesInput(t *testing.T) {
	t.Setenv("INPUT_GITHUB-TOKEN", "input-token")
	t.Setenv("GITHUB_TOKEN", "fallback-token")

	if got := Token(); got != "input-token" {
		t.Fatalf("Token() = %q", got)
	}
}
