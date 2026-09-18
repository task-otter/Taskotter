// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	resolveInputParams struct {
		task           string
		cat            map[string]struct{}
		packageManager config.PackageManager
	}

	nodeVariantExpect struct {
		cat            map[string]struct{}
		packageManager config.PackageManager
		want           string
	}

	nodeConfigErrorCase struct {
		name    string
		pm      config.PackageManager
		wantMsg string
		catalog []string
	}

	nodeVariantWant struct {
		pm   config.PackageManager
		want string
	}

	buildSourceModuleCase struct {
		task   string
		pkgMgr config.PackageManager
		want   string
	}

	nodeToolVariantCase struct {
		moduleName  string
		logicalTask string
		expected    bool
	}

	stripOneSuffixCase struct {
		input        string
		wantResult   string
		wantStripped bool
	}
)

const (
	destPnpm         = "pnpm"
	destNpm          = "npm"
	destYarn         = "yarn"
	destBun          = "bun"
	taskPrettier     = "prettier"
	moduleEslintNpm  = "eslint/node/npm"
	moduleEslintYarn = "eslint/node/yarn"
	errExpected      = "expected error"
	taskESLint       = "eslint"
	srcESLintPnpm    = "eslint/node/pnpm"
	srcESLintBun     = "eslint/bun"
	fmtGotQ          = "got %q"
	fmtUnexpectedErr = "unexpected error: %v"
	missingModule    = "missing"
	pkgDeno          = "deno"
	fmtGotStrippedQ  = "got %q stripped=%t"
	suffixBun        = "/bun"
)
