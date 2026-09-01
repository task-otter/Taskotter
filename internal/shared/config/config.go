// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package config loads and validates the taskotter-sync action configuration.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

type (

	// ValidationError reports invalid action input values.
	ValidationError struct {
		Field   string
		Message string
	}

	// PackageManager selects the Node package manager for JS task resolution.
	PackageManager string

	// Config holds validated TaskOtter action inputs and derived sync metadata.
	Config struct {
		Repository         string
		GitHubOutput       string
		NodePackageManager PackageManager
		BranchName         string
		ConfigurationHash  string
		BaseBranch         string
		StoreVersion       string
		JSRuntime          JSRuntime
		RootTaskfile       string
		TargetFolder       string
		Workspace          string
		GitHubToken        string
		Tasks              []string
		FailOnChanges      bool
		SyncRoot           bool
		IncludesDoc        bool
	}

	hashPayload struct {
		NodePackageManager string
		TargetFolder       string
		StoreVersion       string
		Tasks              []string
		IncludesDoc        bool
		SyncRoot           bool
	}

	parsedEnvInputs struct {
		jsRuntime        JSRuntime
		packageManager   PackageManager
		normalizedTarget string
		rootTaskfile     string
		tasks            []string
		includesDoc      bool
		syncRoot         bool
		failOnChanges    bool
	}

	tasksAndJSSettings struct {
		jsRuntime      JSRuntime
		packageManager PackageManager
		tasks          []string
	}

	jsSettings struct {
		jsRuntime      JSRuntime
		packageManager PackageManager
	}

	toggleFlags struct {
		includesDoc   bool
		syncRoot      bool
		failOnChanges bool
	}

	targetPaths struct {
		normalizedTarget string
		rootTaskfile     string
	}

	rawEnvConfig struct {
		tasksRaw         string
		jsRaw            string
		includesDocRaw   string
		syncRootRaw      string
		failOnChangesRaw string
		storeVersion     string
		targetFolderRaw  string
		rootTaskfileRaw  string
		token            string
		workspace        string
		repository       string
		githubOutput     string
		githubRef        string
		githubBaseRef    string
	}

	assembleConfigInput struct {
		Raw    *rawEnvConfig
		Parsed *parsedEnvInputs
		Hash   string
		Branch string
	}

	mergeParsedArgs struct {
		tasks *tasksAndJSSettings
		flags *toggleFlags
		paths *targetPaths
	}
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
)

var unsafeStoreVersion = regexp.MustCompile(`(?i)(^refs/|\.\./|/|\\|\^|~|\^{commit})`)

// LoadFromEnv reads and validates TaskOtter configuration from GitHub Actions environment variables.
func LoadFromEnv() (*Config, error) {
	raw := loadRawEnv()

	err := validateRuntimeEnv(raw.workspace, raw.token)
	if err != nil {
		return nil, fmt.Errorf("validate runtime environment: %w", err)
	}

	parsed, err := parseEnvInputs(&raw)
	if err != nil {
		return nil, fmt.Errorf("parse environment inputs: %w", err)
	}

	return buildConfig(&raw, &parsed), nil
}

// actionInput reads a GitHub Actions input from the environment.
// Docker container actions expose INPUT_<NAME> with hyphens preserved
// (for example INPUT_GITHUB-TOKEN). Other runners may use underscores
// (for example INPUT_GITHUB_TOKEN).
func actionInput(name string) string {
	upper := strings.ToUpper(name)

	keys := []string{
		"INPUT_" + upper,
		"INPUT_" + strings.ReplaceAll(upper, consts.Hyphen, consts.Underscore),
	}

	for i := range keys {
		if v := strings.TrimSpace(os.Getenv(keys[i])); v != consts.Empty {
			return v
		}
	}

	return consts.Empty
}

func appendTaskLine(line string, seen map[string]struct{}, tasks *[]string) error {
	name, ok, err := processTaskLine(line, seen)
	if err != nil {
		return fmt.Errorf("process task line %q: %w", line, err)
	}

	if ok {
		*tasks = append(*tasks, name)
	}

	return nil
}

func assembleConfig(input *assembleConfigInput) *Config {
	return &Config{
		Tasks:              input.Parsed.tasks,
		JSRuntime:          input.Parsed.jsRuntime,
		NodePackageManager: input.Parsed.packageManager,
		IncludesDoc:        input.Parsed.includesDoc,
		SyncRoot:           input.Parsed.syncRoot,
		FailOnChanges:      input.Parsed.failOnChanges,
		StoreVersion:       input.Raw.storeVersion,
		TargetFolder:       input.Parsed.normalizedTarget,
		RootTaskfile:       input.Parsed.rootTaskfile,
		GitHubToken:        input.Raw.token,
		Workspace:          input.Raw.workspace,
		Repository:         input.Raw.repository,
		GitHubOutput:       input.Raw.githubOutput,
		BaseBranch:         resolveBaseBranch(input.Raw.githubBaseRef, input.Raw.githubRef),
		ConfigurationHash:  input.Hash,
		BranchName:         input.Branch,
	}
}

func buildConfig(raw *rawEnvConfig, parsed *parsedEnvInputs) *Config {
	hash, branch := computeConfigurationHash(buildHashPayload(raw, parsed))

	return assembleConfig(&assembleConfigInput{
		Raw:    raw,
		Parsed: parsed,
		Hash:   hash,
		Branch: branch,
	})
}

func buildHashPayload(raw *rawEnvConfig, parsed *parsedEnvInputs) *hashPayload {
	return &hashPayload{
		Tasks:              parsed.tasks,
		NodePackageManager: string(parsed.packageManager),
		TargetFolder:       parsed.normalizedTarget,
		StoreVersion:       raw.storeVersion,
		IncludesDoc:        parsed.includesDoc,
		SyncRoot:           parsed.syncRoot,
	}
}

func collectTasks(lines []string) ([]string, error) {
	seen := make(map[string]struct{})

	var tasks []string

	for i := range lines {
		err := appendTaskLine(lines[i], seen, &tasks)
		if err != nil {
			return nil, fmt.Errorf("append task line: %w", err)
		}
	}

	return tasks, nil
}

func computeConfigurationHash(payload *hashPayload) (hash, branch string) {
	data, err := json.Marshal(map[string]any{
		"tasks":                payload.Tasks,
		"node_package_manager": payload.NodePackageManager,
		"target_folder":        payload.TargetFolder,
		"store_version":        payload.StoreVersion,
		"includes_doc":         payload.IncludesDoc,
		"sync_root":            payload.SyncRoot,
	})
	if err != nil {
		data = []byte("{}")
	}

	sum := sha256.Sum256(data)

	hash = hex.EncodeToString(sum[:])

	return hash, "taskotter/sync-" + hash[:consts.HashPrefixLen]
}

func ensureNonEmptyTasks(tasks []string) ([]string, error) {
	if len(tasks) == consts.IndexZero {
		return nil, &ValidationError{
			Field:   consts.FieldTasks,
			Message: "at least one task is required",
		}
	}

	return tasks, nil
}

func jsSettingsFromConfig(jsCfg *jsConfig) jsSettings {
	if jsCfg == nil {
		return jsSettings{
			jsRuntime:      consts.Empty,
			packageManager: consts.Empty,
		}
	}

	return jsSettings{
		jsRuntime:      jsCfg.Runtime,
		packageManager: jsCfg.NodePackageManager,
	}
}

func loadGitHubToken() string {
	token := actionInput(consts.FieldGithubToken)

	if token == consts.Empty {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}

	return token
}

func loadRawEnv() rawEnvConfig {
	return rawEnvConfig{
		tasksRaw:         actionInput(consts.FieldTasks),
		jsRaw:            actionInput(consts.FieldJS),
		includesDocRaw:   actionInput(consts.FieldIncludesDoc),
		syncRootRaw:      actionInput(consts.FieldSyncRoot),
		failOnChangesRaw: actionInput(consts.FieldFailOnChanges),
		storeVersion:     actionInput(consts.FieldStoreVersion),
		targetFolderRaw:  actionInput("target-folder"),
		rootTaskfileRaw:  actionInput(consts.FieldRootTaskfile),
		token:            loadGitHubToken(),
		workspace:        os.Getenv(consts.EnvGithubWorkspace),
		repository:       os.Getenv("GITHUB_REPOSITORY"),
		githubOutput:     os.Getenv("GITHUB_OUTPUT"),
		githubRef:        os.Getenv(consts.GitHubRefEnv),
		githubBaseRef:    os.Getenv("GITHUB_BASE_REF"),
	}
}

func mergeParsedInputs(args *mergeParsedArgs) parsedEnvInputs {
	return parsedEnvInputs{
		tasks:            args.tasks.tasks,
		jsRuntime:        args.tasks.jsRuntime,
		packageManager:   args.tasks.packageManager,
		includesDoc:      args.flags.includesDoc,
		syncRoot:         args.flags.syncRoot,
		failOnChanges:    args.flags.failOnChanges,
		normalizedTarget: args.paths.normalizedTarget,
		rootTaskfile:     args.paths.rootTaskfile,
	}
}

func missingActionInput(name string) *ValidationError {
	upper := strings.ToUpper(name)

	return &ValidationError{
		Field: name,
		Message: fmt.Sprintf(
			"is required (set %q in the workflow step; checked env vars INPUT_%s, INPUT_%s)",
			name,
			upper,
			strings.ReplaceAll(upper, consts.Hyphen, consts.Underscore),
		),
	}
}

func normalizeTaskLines(raw string) []string {
	raw = strings.ReplaceAll(raw, ",", consts.Newline)

	return strings.Split(raw, consts.Newline)
}

func parseBoolInput(field, raw string, defaultValue bool) (bool, error) {
	if raw == consts.Empty {
		return defaultValue, nil
	}

	switch strings.ToLower(raw) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, &ValidationError{
			Field:   field,
			Message: fmt.Sprintf("invalid value %q: allowed values are true or false", raw),
		}
	}
}

func parseEnvInputs(raw *rawEnvConfig) (parsedEnvInputs, error) {
	tasksJS, err := parseTasksAndJSSettings(raw)
	if err != nil {
		return parsedEnvInputs{}, fmt.Errorf("parse tasks and js settings: %w", err)
	}

	flags, err := parseToggleFlags(raw)
	if err != nil {
		return parsedEnvInputs{}, fmt.Errorf("parse toggle flags: %w", err)
	}

	target, err := resolveTargetAndTaskfile(raw)
	if err != nil {
		return parsedEnvInputs{}, fmt.Errorf("resolve target and taskfile: %w", err)
	}

	return mergeParsedInputs(&mergeParsedArgs{
		tasks: &tasksJS,
		flags: &flags,
		paths: &target,
	}), nil
}

func parseFailOnChanges(raw string) (bool, error) {
	val, err := parseBoolInput(consts.FieldFailOnChanges, raw, false)
	if err != nil {
		return false, fmt.Errorf(errParseFailOnChanges, err)
	}

	return val, nil
}

func parseIncludesDoc(raw string) (bool, error) {
	val, err := parseBoolInput(consts.FieldIncludesDoc, raw, true)
	if err != nil {
		return false, fmt.Errorf(errParseIncludesDoc, err)
	}

	return val, nil
}

func parseJSSettings(jsRaw string) (jsSettings, error) {
	jsCfg, err := parseJS(jsRaw)
	if err != nil {
		return jsSettings{}, fmt.Errorf("parse js config: %w", err)
	}

	return jsSettingsFromConfig(jsCfg), nil
}

func parseSyncRoot(raw string) (bool, error) {
	val, err := parseBoolInput(consts.FieldSyncRoot, raw, true)
	if err != nil {
		return false, fmt.Errorf(errParseSyncRoot, err)
	}

	return val, nil
}

func parseTasks(raw string) ([]string, error) {
	tasks, err := collectTasks(normalizeTaskLines(raw))
	if err != nil {
		return nil, fmt.Errorf("collect tasks: %w", err)
	}

	nonEmpty, err := ensureNonEmptyTasks(tasks)
	if err != nil {
		return nil, fmt.Errorf("ensure non-empty tasks: %w", err)
	}

	return nonEmpty, nil
}

func parseTasksAndJSSettings(raw *rawEnvConfig) (tasksAndJSSettings, error) {
	tasks, err := parseTasks(raw.tasksRaw)
	if err != nil {
		return tasksAndJSSettings{}, fmt.Errorf("parse tasks: %w", err)
	}

	settings, err := parseJSSettings(raw.jsRaw)
	if err != nil {
		return tasksAndJSSettings{}, fmt.Errorf("parse js settings: %w", err)
	}

	return tasksAndJSSettings{
		tasks:          tasks,
		jsRuntime:      settings.jsRuntime,
		packageManager: settings.packageManager,
	}, nil
}

func parseToggleFlags(raw *rawEnvConfig) (toggleFlags, error) {
	includesDoc, err := parseIncludesDoc(raw.includesDocRaw)
	if err != nil {
		return toggleFlags{}, fmt.Errorf(errParseIncludesDoc, err)
	}

	syncRoot, err := parseSyncRoot(raw.syncRootRaw)
	if err != nil {
		return toggleFlags{}, fmt.Errorf(errParseSyncRoot, err)
	}

	failOnChanges, err := parseFailOnChanges(raw.failOnChangesRaw)
	if err != nil {
		return toggleFlags{}, fmt.Errorf(errParseFailOnChanges, err)
	}

	return toggleFlags{
		includesDoc:   includesDoc,
		syncRoot:      syncRoot,
		failOnChanges: failOnChanges,
	}, nil
}

func processTaskLine(line string, seen map[string]struct{}) (name string, ok bool, err error) {
	name = strings.TrimSpace(line)

	if name == consts.Empty {
		return consts.Empty, false, nil
	}

	err = validateTaskLine(name)
	if err != nil {
		return consts.Empty, false, fmt.Errorf("validate task line: %w", err)
	}

	name, ok = acceptUnseenTask(name, seen)

	return name, ok, nil
}

func acceptUnseenTask(name string, seen map[string]struct{}) (string, bool) {
	existing, seenOK := seen[name]
	iox.Discard(existing)

	if seenOK {
		return consts.Empty, false
	}

	seen[name] = struct{}{}

	return name, true
}

// resolveBaseBranch returns the branch the workflow is operating against.
// Pull request events expose their target branch through GITHUB_BASE_REF;
// push, schedule, and workflow_dispatch events use a refs/heads/... GITHUB_REF.
func resolveBaseBranch(githubBaseRef, githubRef string) string {
	if branch := strings.TrimSpace(githubBaseRef); branch != consts.Empty {
		return branch
	}

	const branchRefPrefix = "refs/heads/"

	if branch, ok := strings.CutPrefix(strings.TrimSpace(githubRef), branchRefPrefix); ok {
		return strings.TrimSpace(branch)
	}

	return consts.Empty
}

func resolveNormalizedTargetFolder(raw *rawEnvConfig) (string, error) {
	targetFolder := DefaultTargetFolder

	if raw.targetFolderRaw != consts.Empty {
		targetFolder = raw.targetFolderRaw
	}

	normalizedTarget, err := pathutil.ValidateTargetFolder(targetFolder, raw.workspace)
	if err != nil {
		return consts.Empty, fmt.Errorf("validate target folder: %w", err)
	}

	return normalizedTarget, nil
}

// resolveRootTaskfile determines where the generated aggregator Taskfile is
// written. When unset it defaults to <targetFolder>/Taskfile.yml; otherwise the
// caller-provided workspace-relative path is validated and must be a YAML file.
func resolveRootTaskfile(raw, targetFolder, workspace string) (string, error) {
	raw = strings.TrimSpace(raw)

	if raw == consts.Empty {
		return pathutil.JoinRelative(targetFolder, consts.Taskfile), nil
	}

	normalized, err := pathutil.ValidateRelativePath(workspace, raw)
	if err != nil {
		return "", fmt.Errorf("validate root-taskfile: %w", err)
	}

	if !strings.HasSuffix(normalized, ".yml") && !strings.HasSuffix(normalized, ".yaml") {
		return "", &ValidationError{
			Field:   consts.FieldRootTaskfile,
			Message: fmt.Sprintf("must be a .yml or .yaml file path, got %q", raw),
		}
	}

	return normalized, nil
}

func resolveTargetAndTaskfile(raw *rawEnvConfig) (targetPaths, error) {
	err := validateStoreVersion(raw.storeVersion)
	if err != nil {
		return targetPaths{}, fmt.Errorf("validate store version: %w", err)
	}

	normalizedTarget, err := resolveNormalizedTargetFolder(raw)
	if err != nil {
		return targetPaths{}, fmt.Errorf("resolve normalized target folder: %w", err)
	}

	rootTaskfile, err := resolveRootTaskfile(raw.rootTaskfileRaw, normalizedTarget, raw.workspace)
	if err != nil {
		return targetPaths{}, fmt.Errorf("resolve root taskfile: %w", err)
	}

	return targetPaths{
		normalizedTarget: normalizedTarget,
		rootTaskfile:     rootTaskfile,
	}, nil
}

func validateRuntimeEnv(workspace, token string) error {
	if workspace == consts.Empty {
		return &ValidationError{Field: consts.EnvGithubWorkspace, Message: "is required"}
	}

	if token == consts.Empty {
		return missingActionInput(consts.FieldGithubToken)
	}

	return nil
}

func validateStoreVersion(version string) error {
	if version == consts.Empty {
		return nil
	}

	if unsafeStoreVersion.MatchString(version) {
		return &ValidationError{
			Field:   consts.FieldStoreVersion,
			Message: fmt.Sprintf("unsafe revision expression %q", version),
		}
	}

	return nil
}

func validateTaskLine(name string) error {
	err := pathutil.ValidateTaskName(name)
	if err != nil {
		return fmt.Errorf("validate task name: %w", err)
	}

	return nil
}

// LockFilePath returns the workspace-relative path to the managed lock file.
func (c *Config) LockFilePath() string {
	return pathutil.JoinRelative(c.TargetFolder, ".taskotter-lock.yml")
}

// MetadataPath returns the workspace-relative path to TaskOtter metadata.
func (c *Config) MetadataPath() string {
	return pathutil.JoinRelative(c.TargetFolder, consts.MetadataPath)
}

// Error implements the error interface, returning the field-prefixed validation message.
func (e *ValidationError) Error() string {
	if e.Field != consts.Empty {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}

	return e.Message
}
