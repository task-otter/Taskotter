// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (

	// DefaultTargetFolder is the workspace-relative directory where synced taskfiles are written.
	DefaultTargetFolder = "taskfiles"

	// StoreRepository is the GitHub repository that hosts TaskOtter store modules.
	StoreRepository = "task-otter/store"

	// LegacyMetadataPath is the pre-migration workspace-relative metadata path.
	LegacyMetadataPath = ".taskotter/metadata.yml"

	// PMNPM is the default npm package manager.
	PMNPM PackageManager = "npm"

	// PMYarn selects Yarn.
	PMYarn PackageManager = "yarn"

	// PMPnpm selects pnpm.
	PMPnpm PackageManager = "pnpm"

	errParseIncludesDoc   = "parse includes-doc: %w"
	errParseSyncRoot      = "parse sync-root: %w"
	errParseFailOnChanges = "parse fail-on-changes: %w"

	fieldJSPackageManager = "js.package-manager"

	fieldJSVersionManager = "js.version-manager"

	// JSRuntimeBun selects Bun as the JS runtime.
	JSRuntimeBun JSRuntime = JSRuntime(consts.Bun)

	// JSRuntimeNodeJS selects Node.js as the JS runtime.
	JSRuntimeNodeJS JSRuntime = "nodejs"
)
