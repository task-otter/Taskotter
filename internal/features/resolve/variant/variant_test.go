// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package variant

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/config"
)

const (
	variantTask = "eslint"
	variantLint = "lint"
)

func TestVariantNames(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		module string
		task   string
		want   bool
	}{
		{name: "npm", module: "eslint/node/npm", task: variantTask, want: true},
		{name: "bun", module: "eslint/bun", task: variantTask, want: true},
		{name: "short", module: "eslint", task: variantTask, want: false},
		{name: "wrong prefix", module: "pre-eslint/node/npm", task: variantTask, want: false},
		{name: "unknown suffix", module: "eslint/node/deno", task: variantTask, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := IsNodeToolVariant(test.module, test.task); got != test.want {
				t.Fatalf("IsNodeToolVariant() = %v", got)
			}
		})
	}
}

func TestBuildSourceModule(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		manager config.PackageManager
		want    string
	}{
		{manager: config.JSRuntimeBun, want: "lint/bun"},
		{manager: config.PMNPM, want: "lint/node/npm"},
		{manager: config.PMYarn, want: "lint/node/yarn"},
		{manager: config.PMPnpm, want: "lint/node/pnpm"},
	} {
		got, err := BuildSourceModule(variantLint, test.manager)

		if err != nil || got != test.want {
			t.Fatalf("BuildSourceModule(%q) = %q, %v", test.manager, got, err)
		}
	}

	if _, err := BuildSourceModule(variantLint, "invalid"); err == nil {
		t.Fatal("invalid package manager accepted")
	}
}

func TestStripOneSuffix(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "npm", in: "lint/node/npm", want: variantLint, ok: true},
		{name: "bun", in: "lint/bun", want: variantLint, ok: true},
		{name: "plain", in: variantLint, want: variantLint, ok: false},
		{name: "suffix only", in: "/bun", want: "/bun", ok: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, ok := StripOneSuffix(test.in)

			if got != test.want || ok != test.ok {
				t.Fatalf("StripOneSuffix() = %q, %v", got, ok)
			}
		})
	}
}
