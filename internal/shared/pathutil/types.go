// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil

type (
	// FieldError is a path validation error with a field name.
	FieldError interface {
		error
		FieldName() string
	}

	// PathError reports invalid path or task name configuration.
	//
	// Field names the config key; Value is the rejected input; Message explains why.
	PathError struct {
		Field   string
		Value   string
		Message string
	}

	insideRootParams = struct {
		base       string
		normalized string
		raw        string
		field      string
		outsideMsg string
	}

	pathComponentContext = struct {
		evalWorkspace string
		current       string
		raw           string
	}
)
