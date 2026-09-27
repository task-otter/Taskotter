// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil

const (
	errPathOutsideRootMsg = "path resolves outside root"

	fieldTasks         = "tasks"
	fieldTargetFolder  = "target-folder"
	fieldPath          = "path"
	taskNamePatternMsg = "invalid task name %q: must match ^[a-z0-9][a-z0-9-]*$"

	errMustBeRelativePath      = "must be a relative path"
	errMustNotBeEmptyAfterNorm = "must not be empty after normalization"
	errMustNotContainDotDot    = "must not contain .. path components"
	errResolveRoot             = "resolve root: %w"
	errNormalizeTargetFolder   = "normalize target folder: %w"
	errValidateRelativePath    = "validate relative path %q: %w"
	errResolveValidatedRoot    = "resolve validated root: %w"
	errFmtOpenFile             = "open file %q: %w"
)
