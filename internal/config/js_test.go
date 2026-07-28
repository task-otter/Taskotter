package config_test

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
)

func TestParseJSNodeJSDefaults(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_JS"] = "runtime: nodejs\n"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		t.Fatalf("JSRuntime = %q, want nodejs", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.PMNPM {
		t.Fatalf("NodePackageManager = %q, want npm", cfg.NodePackageManager)
	}

	if cfg.NodeVersionManager != config.VMFnm {
		t.Fatalf("NodeVersionManager = %q, want fnm", cfg.NodeVersionManager)
	}
}

func TestParseJSBun(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_JS"] = "runtime: bun\n"
	setEnv(t, env)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JSRuntime != config.JSRuntimeBun {
		t.Fatalf("JSRuntime = %q, want bun", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != config.PMBun {
		t.Fatalf("NodePackageManager = %q, want bun", cfg.NodePackageManager)
	}

	if cfg.NodeVersionManager != "" {
		t.Fatalf("NodeVersionManager = %q, want empty", cfg.NodeVersionManager)
	}
}

func TestParseJSBunRejectsVersionManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_JS"] = "runtime: bun\nversion-manager: fnm\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseJSNodeJSRejectsBunPackageManager(t *testing.T) {
	dir := t.TempDir()
	env := baseEnv(dir)
	env["INPUT_JS"] = "runtime: nodejs\npackage-manager: bun\n"
	setEnv(t, env)

	_, err := config.LoadFromEnv()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseJSEmpty(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, baseEnv(dir))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JSRuntime != "" {
		t.Fatalf("JSRuntime = %q, want empty", cfg.JSRuntime)
	}

	if cfg.NodePackageManager != "" {
		t.Fatalf("NodePackageManager = %q, want empty", cfg.NodePackageManager)
	}
}
