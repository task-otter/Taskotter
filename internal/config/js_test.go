// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
)

func jsEnv(dir, jsValue string) map[string]string {
	env := baseEnv(dir)

	env[consts.InputJS] = jsValue

	return env
}

// TestParseJSNodeJSDefaults verifies nodejs runtime defaults to npm package manager and fnm.
func TestParseJSNodeJSDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg := loadEnvOK(t, jsEnv(dir, "runtime: nodejs\n"))

	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		t.Fatalf(fmtJSRuntimeWantNodeJS, cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.PMNPM {
		t.Fatalf("NodePackageManager = %q, want npm", cfg.NodePackageManager)
	}

	if cfg.NodeVersionManager != config.VMFnm {
		t.Fatalf(fmtWantFnm, cfg.NodeVersionManager)
	}
}

// TestParseJSBun verifies bun runtime sets bun package manager with no version manager.
func TestParseJSBun(t *testing.T) {
	dir := t.TempDir()
	cfg := loadEnvOK(t, jsEnv(dir, "runtime: bun\n"))

	if cfg.JSRuntime != config.JSRuntimeBun {
		t.Fatalf("JSRuntime = %q, want bun", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.PMBun {
		t.Fatalf("NodePackageManager = %q, want bun", cfg.NodePackageManager)
	}

	if cfg.NodeVersionManager != consts.Empty {
		t.Fatalf("NodeVersionManager = %q, want empty", cfg.NodeVersionManager)
	}
}

// TestParseJSBunRejectsVersionManager verifies bun runtime with a version-manager fails validation.
func TestParseJSBunRejectsVersionManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: bun\nversion-manager: fnm\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal(consts.ExpectedValidErr)
	}
}

// TestParseJSBunRejectsPackageManager verifies bun runtime with an explicit package-manager fails.
func TestParseJSBunRejectsPackageManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: bun\npackage-manager: pnpm\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal(consts.ExpectedValidErr)
	}
}

// TestParseJSNodeJSRejectsBunPackageManager verifies nodejs runtime rejects the bun package manager.
func TestParseJSNodeJSRejectsBunPackageManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: nodejs\npackage-manager: bun\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal(consts.ExpectedValidErr)
	}
}

// TestParseJSEmpty verifies an empty js input leaves runtime and package manager unset.
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
func TestParseJSDefaultsRuntimeToNodeJS(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "package-manager: yarn\nversion-manager: nvm\n"
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
func TestParseJSRejectsInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = ":"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid YAML error")
	}
}

// TestParseJSRejectsInvalidRuntime verifies an unrecognized runtime value is rejected.
func TestParseJSRejectsInvalidRuntime(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: deno\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid runtime error")
	}
}

// TestParseJSRejectsInvalidVersionManager verifies an unrecognized version manager value is rejected.
func TestParseJSRejectsInvalidVersionManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)

	env[consts.InputJS] = "runtime: nodejs\nversion-manager: volta\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid version manager error")
	}
}
