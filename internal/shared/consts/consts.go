// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package consts holds shared string, path, and format constants used across TaskOtter.
package consts

const (

	// Empty string constant to avoid literal repetition.
	Empty = ""

	// PathSepString is the forward-slash path separator used in normalized paths.
	PathSepString = "/"
	// PathParent is the ".." parent directory path component.
	PathParent = ".."
	// GitDir is the name of the .git directory.
	GitDir = ".git"
	// GitDirWithSep is the .git directory name with a trailing separator.
	GitDirWithSep = ".git/"
	// GitHubActions is the path to the .github/actions directory.
	GitHubActions = ".github/actions"
	// GitHubActsWith is the .github/actions path with a trailing separator.
	GitHubActsWith = ".github/actions/"
	// ReadmeMD is the README.md filename.
	ReadmeMD = "README.md"
	// Taskfile is the default Taskfile.yml filename.
	Taskfile = "Taskfile.yml"
	// TaskfileSuffix is the Taskfile.yml filename with a leading separator.
	TaskfileSuffix = "/Taskfile.yml"
	// Metadata is the module metadata.yml filename.
	Metadata = "metadata.yml"
	// MetadataPath is the workspace-relative path to the TaskOtter metadata file.
	MetadataPath = ".taskotter/metadata.yml"
	// TaskotterDir is the .taskotter directory name.
	TaskotterDir = ".taskotter"
	// TaskotterDirSep is the .taskotter directory name with a trailing separator.
	TaskotterDirSep = ".taskotter/"

	// GitOrigin is the name of the default git remote.
	GitOrigin = "origin"
	// GitRevParse is the git rev-parse subcommand name.
	GitRevParse = "rev-parse"
	// GitConfig is the git config subcommand name.
	GitConfig = "config"
	// GitAdd is the git add subcommand name.
	GitAdd = "add"
	// GitInit is the git init subcommand name.
	GitInit = "init"
	// GitBranchFlag is the -b flag used to name a branch.
	GitBranchFlag = "-b"
	// GitRemoteHeadRef is the ref path for the origin remote's HEAD.
	GitRemoteHeadRef = "refs/remotes/origin/HEAD"
	// GitRefsRemoteOrg is the ref path prefix for origin remote branches.
	GitRefsRemoteOrg = "refs/remotes/origin/"
	// Newline is a single newline character.
	Newline = "\n"
	// DoubleSlash is two consecutive forward slashes.
	DoubleSlash = "//"

	// BranchFmtErr formats an assertion failure about the expected branch name.
	BranchFmtErr = "branch = %q, want main"
	// FormatErr formats a LoadFromEnv error message.
	FormatErr = "LoadFromEnv() error = %v"
	// UnexpectedErr formats an unexpected error message.
	UnexpectedErr = "unexpected error: %v"
	// ExpectedErr is the message used when an error was expected but not returned.
	ExpectedErr = "expected error"
	// ExpectedValidErr is the message used when a validation error was expected but not returned.
	ExpectedValidErr = "expected validation error"
	// ListFmt formats a single markdown list item line.
	ListFmt = "  - `%s`\n"
	// ResolveRootErr formats a root resolution error message.
	ResolveRootErr = "resolve root: %w"
	// GotStripped formats a got-value/stripped-flag assertion message.
	GotStripped = "got %q stripped=%t"

	// GitHubRefEnv is the GITHUB_REF environment variable name.
	GitHubRefEnv = "GITHUB_REF"
	// InputGithubToken is the INPUT_GITHUB_TOKEN environment variable name.
	InputGithubToken = "INPUT_GITHUB_" + "TOKEN"
	// InputTasks is the INPUT_TASKS environment variable name.
	InputTasks = "INPUT_TASKS"
	// InputJS is the INPUT_JS environment variable name.
	InputJS = "INPUT_JS"
	// InputStoreVersion is the INPUT_STORE_VERSION environment variable name.
	InputStoreVersion = "INPUT_STORE_VERSION"
	// InputTargetFolder is the INPUT_TARGET_FOLDER environment variable name.
	InputTargetFolder = "INPUT_TARGET_FOLDER"

	// Go is the go runtime/language identifier.
	Go = "go"
	// Bun is the bun runtime identifier.
	Bun = "bun"
	// NodePkgMgr is the node/fnm/npm variant module identifier.
	NodePkgMgr = "node/fnm/npm"
	// NodePkgMgrPnpm is the node/fnm/pnpm variant module identifier.
	NodePkgMgrPnpm = "node/fnm/pnpm"
	// EslintBun is the eslint/bun variant module identifier.
	EslintBun = "eslint/bun"
	// EslintNode is the eslint module identifier.
	EslintNode = "eslint"

	// IncludesKey is the includes keyword for taskfiles.
	IncludesKey = "includes"
	// VarsKey is the vars keyword for taskfiles.
	VarsKey = "vars"
	// TaskfileKey is the taskfile keyword used in include entries.
	TaskfileKey = "taskfile"

	// FilePerm644 is the octal file permission for regular readable/writable files.
	FilePerm644 = 0o644
	// FilePerm755 is the octal file permission for readable/writable/executable files.
	FilePerm755 = 0o755
	// FilePerm111 is the octal file permission bits for execute-only access.
	FilePerm111 = 0o111
	// FilePerm4755 is the octal file permission including the setuid bit.
	FilePerm4755 = 0o4755

	// SkipfilesPath is the workspace-relative path to the internal skipfiles list.
	SkipfilesPath = "internal/skipfiles"

	// FieldRootTaskfile is the root-taskfile config field name.
	FieldRootTaskfile = "root-taskfile"
	// FieldGithubToken is the github-token config field name.
	FieldGithubToken = "github-" + "token"
	// FieldTasks is the tasks config field name.
	FieldTasks = "tasks"
	// FieldJS is the js config field name.
	FieldJS = "js"
	// FieldIncludesDoc is the includes-doc config field name.
	FieldIncludesDoc = "includes-doc"
	// FieldSyncRoot is the sync-root config field name.
	FieldSyncRoot = "sync-root"
	// FieldFailOnChanges is the fail-on-changes config field name.
	FieldFailOnChanges = "fail-on-changes"
	// FieldStoreVersion is the store-version config field name.
	FieldStoreVersion = "store-version"
	// EnvGithubWorkspace is the GITHUB_WORKSPACE environment variable name.
	EnvGithubWorkspace = "GITHUB_WORKSPACE"
	// Hyphen is a single hyphen character.
	Hyphen = "-"
	// Underscore is a single underscore character.
	Underscore = "_"

	// JSValidOnlyForNodejs is shared validation message text.
	JSValidOnlyForNodejs = "is only valid when js.runtime is nodejs"

	// HashPrefixLen is the number of hex characters used from the configuration hash.
	HashPrefixLen = 12

	// IndexZero is the zero index for slice and array access.
	IndexZero = 0
	// IndexOne is the one index for slice and array access.
	IndexOne = 1
	// IndexTwo is the two index for slice and array access.
	IndexTwo = 2
	// IndexThree is the three index for slice and array access.
	IndexThree = 3
	// IndexSeven is the seven index for slice and array access.
	IndexSeven = 7
	// IndexNine is the nine index for slice and array access.
	IndexNine = 9
	// Index99 is the ninety-nine index for slice and array access.
	Index99 = 99
	// Index256 is the two-hundred-fifty-six index for slice and array access.
	Index256 = 256
	// Index42 is the forty-two index for slice and array access.
	Index42 = 42
)
