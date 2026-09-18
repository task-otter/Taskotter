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
	errRenderSection      = "render %s section: %w"
	errRenderMappingPair  = "render mapping pair: %w"
	errScalarSourceEnd    = "scalar source end: %w"
	errFindClosingQuote   = "find closing quote: %w"
	defaultMarker         = "| default"

	lineFeed               = "\n"
	carriageReturnLineFeed = "\r\n"
	documentStartLineFeed  = "---\n"
	documentStartCRLF      = "---\r\n"
	nodeStartOffsetError   = "node start offset: %w"
	lineStartOffsetError   = "line start offset: %w"
	versionKey             = "version"
	rootVarIndent          = 4
	mappingPairIndent      = yamlMappingPairKeyValue
)
