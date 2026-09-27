// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

//nolint:distance // This parser package deliberately exposes only configuration primitives.
package jsconfig

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	fieldJSPackageManager = "js.package-manager"

	fieldJSVersionManager = "js.version-manager"

	// JSRuntimeBun selects Bun as the JS runtime.
	JSRuntimeBun JSRuntime = JSRuntime(consts.Bun)

	// JSRuntimeNodeJS selects Node.js as the JS runtime.
	JSRuntimeNodeJS JSRuntime = "nodejs"
)
