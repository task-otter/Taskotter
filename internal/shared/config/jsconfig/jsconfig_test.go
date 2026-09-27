// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package jsconfig

import (
	"errors"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	parseCase struct {
		name        string
		raw         string
		wantRuntime JSRuntime
		wantManager PackageManager
	}
)

// TestParseValidSettings verifies the behavior covered by this test.
//
//nolint:funlen,maintidx // The table cases are intentionally written in full for parser coverage.
func TestParseValidSettings(t *testing.T) {
	t.Parallel()

	cases := []parseCase{
		{name: "empty", raw: "", wantRuntime: consts.Empty, wantManager: consts.Empty},
		{
			name:        "default node",
			raw:         "{}",
			wantRuntime: JSRuntimeNodeJS,
			wantManager: packageManagerNPM,
		},
		{
			name:        "node pnpm",
			raw:         "runtime: nodejs\npackage-manager: pnpm\n",
			wantRuntime: JSRuntimeNodeJS,
			wantManager: packageManagerPNPM,
		},
		{name: "bun", raw: "runtime: bun\n", wantRuntime: JSRuntimeBun, wantManager: JSRuntimeBun},
	}

	for idx := range cases {
		testCase := &cases[idx]

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertValidSettings(t, testCase)
		})
	}
}

func assertValidSettings(t *testing.T, testCase *parseCase) {
	t.Helper()

	got, err := Parse(testCase.raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got.Runtime != testCase.wantRuntime || got.NodePackageManager != testCase.wantManager {
		t.Fatalf("Parse() = %#v", got)
	}
}

// TestParseValidationErrors verifies the behavior covered by this test.
//
//nolint:funlen,maintidx // The table cases are intentionally written in full for parser coverage.
func TestParseValidationErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		raw       string
		wantField string
	}{
		{name: "invalid yaml", raw: ":", wantField: consts.FieldJS},
		{name: "invalid runtime", raw: "runtime: deno\n", wantField: fieldJSRuntime},
		{
			name:      "invalid package manager",
			raw:       "package-manager: bun\n",
			wantField: fieldJSPackageManager,
		},
		{
			name:      "bun package manager",
			raw:       "runtime: bun\npackage-manager: npm\n",
			wantField: fieldJSPackageManager,
		},
		{
			name:      "version manager",
			raw:       "version-manager: nodenv\n",
			wantField: fieldJSVersionManager,
		},
	}

	for i := range cases {
		tc := cases[i]

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assertValidationError(t, tc.raw, tc.wantField)
		})
	}
}

func assertValidationError(t *testing.T, raw, wantField string) {
	t.Helper()

	validationErr := parseValidationError(t, raw)

	if validationErr.FieldName() != wantField {
		t.Fatalf("field = %q, want %q", validationErr.FieldName(), wantField)
	}

	if validationErr.Error() == consts.Empty {
		t.Fatal("empty validation error")
	}
}

func parseValidationError(t *testing.T, raw string) *ValidationError {
	t.Helper()

	parsed, err := Parse(raw)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if parsed != nil {
		t.Fatalf("Parse() config = %#v, want nil", parsed)
	}

	var validationErr *ValidationError

	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T %[1]v", err)
	}

	return validationErr
}
