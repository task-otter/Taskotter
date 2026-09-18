// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

const (
	rootTaskfileVersion     = "3.5"
	rootTemplate            = "---\nversion: \"" + rootTaskfileVersion + "\"\n"
	yamlMappingPairKeyValue = 2

	dotSlash       = "./"
	dotDotSlash    = "../"
	keyIncludes    = "includes"
	keyTaskfile    = "taskfile"
	keyDir         = "dir"
	keyVars        = "vars"
	keyTasks       = "tasks"
	taskfileSuffix = "Taskfile.yml"
	doubleQuote    = `"`

	errParseTaskfileRoot  = "parse taskfile root: %w"
	errMarshalAndValidate = "marshal and validate: %w"
)
