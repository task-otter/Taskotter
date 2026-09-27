// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	rootTaskfileVersion     = "3.5"
	rootTemplate            = "---\nversion: \"" + rootTaskfileVersion + "\"\n"
	yamlMappingPairKeyValue = consts.IndexTwo

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
	rawVarPlaceholderPref = "TASKOTTER_RAW_VAR_"
	rawVarPlaceholderSuf  = "_Z"
	yamlNewlineCutset     = "\r\n"
)
