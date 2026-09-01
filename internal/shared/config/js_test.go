// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

func jsEnv(dir, jsValue string) map[string]string {
	env := baseEnv(dir)

	env[consts.InputJS] = jsValue

	return env
}

// TestParseJSNodeJSDefaults verifies nodejs runtime defaults to the npm package manager.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSNodeJSDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg := loadEnvOK(t, jsEnv(dir, "runtime: nodejs\n"))

	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		t.Fatalf(fmtJSRuntimeWantNodeJS, cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.PMNPM {
		t.Fatalf("NodePackageManager = %q, want npm", cfg.NodePackageManager)
	}
}

// TestParseJSBun verifies bun runtime sets the bun package manager.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSBun(t *testing.T) {
	dir := t.TempDir()
	cfg := loadEnvOK(t, jsEnv(dir, "runtime: bun\n"))

	if cfg.JSRuntime != config.JSRuntimeBun {
		t.Fatalf("JSRuntime = %q, want bun", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.PackageManager(config.JSRuntimeBun) {
		t.Fatalf("NodePackageManager = %q, want bun", cfg.NodePackageManager)
	}
}

// TestParseJSBunRejectsVersionManager verifies the removed version-manager key fails under bun.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSBunRejectsVersionManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = testJSBunWithFnm
	loadEnvExpectError(t, env, consts.ExpectedValidErr)
}

// TestParseJSBunRejectsPackageManager verifies bun runtime with an explicit package-manager fails.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSBunRejectsPackageManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: bun\npackage-manager: pnpm\n"
	loadEnvExpectError(t, env, consts.ExpectedValidErr)
}

// TestParseJSNodeJSRejectsBunPackageManager verifies nodejs runtime rejects the bun package manager.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSNodeJSRejectsBunPackageManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: nodejs\npackage-manager: bun\n"
	loadEnvExpectError(t, env, consts.ExpectedValidErr)
}

// TestParseJSEmpty verifies an empty js input leaves runtime and package manager unset.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSEmpty(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, baseEnv(dir))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JSRuntime != consts.Empty {
		t.Fatalf("JSRuntime = %q, want empty", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != consts.Empty {
		t.Fatalf("NodePackageManager = %q, want empty", cfg.NodePackageManager)
	}
}

// TestParseJSDefaultsRuntimeToNodeJS verifies the runtime defaults to nodejs when unset.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSDefaultsRuntimeToNodeJS(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "package-manager: yarn\n"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		t.Fatalf(fmtJSRuntimeWantNodeJS, cfg.JSRuntime)
	}
}

// TestParseJSRejectsInvalidYAML verifies malformed js YAML input is rejected.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSRejectsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = ":"
	loadEnvExpectError(t, env, "expected invalid YAML error")
}

// TestParseJSRejectsInvalidRuntime verifies an unrecognized runtime value is rejected.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSRejectsInvalidRuntime(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: deno\n"
	loadEnvExpectError(t, env, "expected invalid runtime error")
}

// TestParseJSRejectsVersionManager verifies the removed version-manager key is rejected outright,
// including values that were valid before the store dropped its fnm and nvm variants.
//
//nolint:paralleltest // LoadFromEnv uses t.Setenv; cannot run in parallel
func TestParseJSRejectsVersionManager(t *testing.T) {
	removed := []string{"fnm", "nvm", "volta"}

	for i := range removed {
		dir := t.TempDir()
		env := baseEnv(dir)

		env[consts.InputJS] = "runtime: nodejs\nversion-manager: " + removed[i] + "\n"
		loadEnvExpectError(t, env, "expected removed version-manager error")
	}
}
