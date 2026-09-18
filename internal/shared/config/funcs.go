// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config

import (
	"fmt"
	"os"
	"strings"

	configenv "github.com/task-otter/Taskotter/internal/shared/config/env"
	confighash "github.com/task-otter/Taskotter/internal/shared/config/hash"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

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
	return configenv.Input(name)
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
		NodePackageManager: parsed.packageManager,
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
	input := confighash.Input{
		Tasks:              payload.Tasks,
		NodePackageManager: payload.NodePackageManager,
		TargetFolder:       payload.TargetFolder,
		StoreVersion:       payload.StoreVersion,
		IncludesDoc:        payload.IncludesDoc,
		SyncRoot:           payload.SyncRoot,
	}
	hash, branch, err := confighash.Compute(&input, consts.HashPrefixLen)
	iox.Discard(err)

	return hash, branch
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

// jsSettingsFromConfig converts a parsed js config; parseJS always returns a
// non-nil config when it reports no error, so jsCfg is never nil here.
func jsSettingsFromConfig(jsCfg *jsConfig) jsSettings {
	return jsSettings{
		jsRuntime:      jsCfg.Runtime,
		packageManager: jsCfg.NodePackageManager,
	}
}

func loadGitHubToken() string {
	return configenv.Token()
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
func LockFilePath(cfg *Config) string {
	return pathutil.JoinRelative(cfg.TargetFolder, ".taskotter-lock.yml")
}

// MetadataPath returns the workspace-relative path to TaskOtter metadata.
func MetadataPath(cfg *Config) string {
	return pathutil.JoinRelative(cfg.TargetFolder, consts.MetadataPath)
}

// Error implements the error interface, returning the field-prefixed validation message.
func (e *ValidationError) Error() string {
	if e.Field != consts.Empty {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}

	return e.Message
}

// FieldName returns the invalid configuration field.
func (e *ValidationError) FieldName() string {
	if e.Message == consts.Empty {
		return e.Field
	}

	return e.Field
}

func jsRuntimeParsers() map[JSRuntime]func(*jsInput) (*jsConfig, error) {
	return map[JSRuntime]func(*jsInput) (*jsConfig, error){
		JSRuntimeBun:    parseJSBun,
		JSRuntimeNodeJS: parseJSNodeJS,
	}
}

func defaultedJSRuntime(rawRuntime string) string {
	runtime := strings.TrimSpace(rawRuntime)

	if runtime == consts.Empty {
		runtime = JSRuntimeNodeJS
	}

	return runtime
}

func defaultedRaw(raw, fallback string) string {
	trimmed := strings.TrimSpace(raw)

	if trimmed == consts.Empty {
		return fallback
	}

	return trimmed
}

func dispatchJSRuntime(yamlInput *jsInput) (*jsConfig, error) {
	err := rejectVersionManager(yamlInput)
	if err != nil {
		return nil, fmt.Errorf("reject js version manager: %w", err)
	}

	jsCfg, err := parseWithJSRuntime(yamlInput, defaultedJSRuntime(yamlInput.Runtime))
	if err != nil {
		return nil, fmt.Errorf("parse js runtime config: %w", err)
	}

	return jsCfg, nil
}

func parseWithJSRuntime(yamlInput *jsInput, runtime string) (*jsConfig, error) {
	parser, ok := jsRuntimeParsers()[runtime]

	if !ok {
		return nil, &ValidationError{
			Field:   "js.runtime",
			Message: fmt.Sprintf("invalid value %q: allowed values are bun or nodejs", runtime),
		}
	}

	jsCfg, err := parser(yamlInput)
	if err != nil {
		return nil, fmt.Errorf("parse %s config: %w", runtime, err)
	}

	return jsCfg, nil
}

// rejectVersionManager fails on the removed js.version-manager key. Store modules no longer
// carry a version-manager path segment, so the value has no meaning.
func rejectVersionManager(yamlInput *jsInput) error {
	if strings.TrimSpace(yamlInput.VersionManager) == consts.Empty {
		return nil
	}

	return &ValidationError{
		Field:   fieldJSVersionManager,
		Message: consts.JSVersionManagerRemoved,
	}
}

func emptyJSConfig() *jsConfig {
	return &jsConfig{
		Runtime:            consts.Empty,
		NodePackageManager: consts.Empty,
	}
}

func parseJS(raw string) (*jsConfig, error) {
	raw = strings.TrimSpace(raw)

	if raw == consts.Empty {
		return emptyJSConfig(), nil
	}

	yamlInput, err := parseJSYAML(raw)
	if err != nil {
		return nil, fmt.Errorf("parse js yaml: %w", err)
	}

	jsCfg, err := dispatchJSRuntime(&yamlInput)
	if err != nil {
		return nil, fmt.Errorf("dispatch js runtime: %w", err)
	}

	return jsCfg, nil
}

func parseJSBun(yamlInput *jsInput) (*jsConfig, error) {
	if strings.TrimSpace(yamlInput.PackageManager) != consts.Empty {
		return nil, &ValidationError{
			Field:   fieldJSPackageManager,
			Message: consts.JSValidOnlyForNodejs,
		}
	}

	return &jsConfig{
		Runtime:            JSRuntimeBun,
		NodePackageManager: JSRuntimeBun,
	}, nil
}

func parseJSInput(raw string) (jsInput, error) {
	fields := make(map[string]string)

	err := yaml.Unmarshal([]byte(raw), &fields)
	if err != nil {
		return jsInput{}, fmt.Errorf("parse js input: %w", err)
	}

	return jsInput{
		Runtime:        fields["runtime"],
		PackageManager: fields["package-manager"],
		VersionManager: fields["version-manager"],
	}, nil
}

func parseJSNodeJS(yamlInput *jsInput) (*jsConfig, error) {
	packageManagerRaw := defaultedRaw(yamlInput.PackageManager, PMNPM)

	packageManager, err := validatePackageManager(packageManagerRaw)
	if err != nil {
		return nil, fmt.Errorf("validate package manager: %w", err)
	}

	return &jsConfig{
		Runtime:            JSRuntimeNodeJS,
		NodePackageManager: packageManager,
	}, nil
}

func parseJSYAML(raw string) (jsInput, error) {
	yamlInput, err := parseJSInput(raw)
	if err != nil {
		return jsInput{}, &ValidationError{
			Field:   consts.FieldJS,
			Message: fmt.Sprintf("invalid YAML: %v", err),
		}
	}

	return yamlInput, nil
}

func parseNodePackageManager(raw string) (PackageManager, error) {
	switch raw {
	case "npm", "yarn", "pnpm":
		return raw, nil
	default:
		return consts.Empty, &ValidationError{
			Field:   fieldJSPackageManager,
			Message: fmt.Sprintf("invalid value %q: allowed values are npm, yarn, or pnpm", raw),
		}
	}
}

// validatePackageManager accepts only the node package managers; "bun" is rejected
// by parseNodePackageManager and must be selected through js.runtime instead.
func validatePackageManager(raw string) (PackageManager, error) {
	packageManager, err := parseNodePackageManager(raw)
	if err != nil {
		return consts.Empty, fmt.Errorf("parse node package manager: %w", err)
	}

	return packageManager, nil
}
