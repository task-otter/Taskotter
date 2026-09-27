// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package jsconfig

type (

	// JSRuntime selects the JavaScript runtime for Node-oriented task resolution.
	JSRuntime = string

	// PackageManager selects the Node package manager.
	PackageManager = string

	// ValidationError reports an invalid JavaScript input field.
	ValidationError struct {
		Field   string
		Message string
	}

	jsInput = struct {
		Runtime        string
		PackageManager string
		VersionManager string
	}

	// Settings is the validated JavaScript runtime configuration.
	Settings = struct {
		Runtime            JSRuntime
		NodePackageManager PackageManager
	}
)
