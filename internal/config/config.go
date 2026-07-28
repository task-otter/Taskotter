// Package config loads and validates TaskOtter GitHub Action inputs from the environment.
package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/task-otter/Taskotter/internal/pathutil"
)

const (
	// DefaultTargetFolder is the workspace-relative directory where synced taskfiles are written.
	DefaultTargetFolder = "taskfiles"
	// StoreRepository is the GitHub repository that hosts TaskOtter store modules.
	StoreRepository = "task-otter/store"
	// LegacyMetadataPath is the pre-migration workspace-relative metadata path.
	LegacyMetadataPath = ".taskotter/metadata.yml"
)

var unsafeStoreVersion = regexp.MustCompile(`(?i)(^refs/|\.\./|/|\\|\^|~|\^{commit})`)

// ValidationError reports invalid action input values.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}

	return e.Message
}

// actionInput reads a GitHub Actions input from the environment.
// Docker container actions expose INPUT_<NAME> with hyphens preserved
// (for example INPUT_GITHUB-TOKEN). Other runners may use underscores
// (for example INPUT_GITHUB_TOKEN).
func actionInput(name string) string {
	upper := strings.ToUpper(name)
	for _, key := range []string{
		"INPUT_" + upper,
		"INPUT_" + strings.ReplaceAll(upper, "-", "_"),
	} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}

	return ""
}

func missingActionInput(name string) *ValidationError {
	upper := strings.ToUpper(name)

	return &ValidationError{
		Field: name,
		Message: fmt.Sprintf(
			"is required (set %q in the workflow step; checked env vars INPUT_%s, INPUT_%s)",
			name,
			upper,
			strings.ReplaceAll(upper, "-", "_"),
		),
	}
}

// PackageManager selects the Node package manager for JS task resolution.
type PackageManager string

const (
	// PMNPM is the default npm package manager.
	PMNPM PackageManager = "npm"
	// PMYarn selects Yarn.
	PMYarn PackageManager = "yarn"
	// PMPnpm selects pnpm.
	PMPnpm PackageManager = "pnpm"
	// PMBun selects Bun as the runtime package manager.
	PMBun PackageManager = "bun"
)

// VersionManager selects the Node version manager for JS task resolution.
type VersionManager string

const (
	// VMFnm selects fnm as the Node version manager.
	VMFnm VersionManager = "fnm"
	// VMNvm selects nvm as the Node version manager.
	VMNvm VersionManager = "nvm"
)

// Config holds validated TaskOtter action inputs and derived sync metadata.
type Config struct {
	Tasks              []string
	JSRuntime          JSRuntime
	NodePackageManager PackageManager
	NodeVersionManager VersionManager
	IncludesDoc        bool
	SyncRoot           bool
	FailOnChanges      bool
	StoreVersion       string
	TargetFolder       string
	RootTaskfile       string
	GitHubToken        string
	Workspace          string
	Repository         string
	GitHubOutput       string
	BaseBranch         string
	ConfigurationHash  string
	BranchName         string
}

type hashPayload struct {
	Tasks              []string
	NodePackageManager string
	NodeVersionManager string
	TargetFolder       string
	StoreVersion       string
	IncludesDoc        bool
	SyncRoot           bool
}

// LoadFromEnv reads and validates TaskOtter configuration from GitHub Actions environment variables.
func LoadFromEnv() (*Config, error) {
	raw := loadRawEnv()

	err := validateRuntimeEnv(raw.workspace, raw.token)
	if err != nil {
		return nil, err
	}

	parsed, err := parseEnvInputs(raw)
	if err != nil {
		return nil, err
	}

	hash, branch := computeConfigurationHash(hashPayload{
		Tasks:              parsed.tasks,
		NodePackageManager: string(parsed.packageManager),
		NodeVersionManager: string(parsed.versionManager),
		TargetFolder:       parsed.normalizedTarget,
		StoreVersion:       raw.storeVersion,
		IncludesDoc:        parsed.includesDoc,
		SyncRoot:           parsed.syncRoot,
	})

	return &Config{
		Tasks:              parsed.tasks,
		JSRuntime:          parsed.jsRuntime,
		NodePackageManager: parsed.packageManager,
		NodeVersionManager: parsed.versionManager,
		IncludesDoc:        parsed.includesDoc,
		SyncRoot:           parsed.syncRoot,
		FailOnChanges:      parsed.failOnChanges,
		StoreVersion:       raw.storeVersion,
		TargetFolder:       parsed.normalizedTarget,
		RootTaskfile:       parsed.rootTaskfile,
		GitHubToken:        raw.token,
		Workspace:          raw.workspace,
		Repository:         raw.repository,
		GitHubOutput:       raw.githubOutput,
		BaseBranch:         resolveBaseBranch(raw.githubBaseRef, raw.githubRef),
		ConfigurationHash:  hash,
		BranchName:         branch,
	}, nil
}

type parsedEnvInputs struct {
	tasks            []string
	jsRuntime        JSRuntime
	packageManager   PackageManager
	versionManager   VersionManager
	includesDoc      bool
	syncRoot         bool
	failOnChanges    bool
	normalizedTarget string
	rootTaskfile     string
}

func parseEnvInputs(raw rawEnvConfig) (parsedEnvInputs, error) {
	tasks, err := parseTasks(raw.tasksRaw)
	if err != nil {
		return parsedEnvInputs{}, err
	}

	jsCfg, err := parseJS(raw.jsRaw)
	if err != nil {
		return parsedEnvInputs{}, err
	}

	jsRuntime, packageManager, versionManager := jsSettingsFromConfig(jsCfg)

	includesDoc, err := parseIncludesDoc(raw.includesDocRaw)
	if err != nil {
		return parsedEnvInputs{}, err
	}

	syncRoot, err := parseSyncRoot(raw.syncRootRaw)
	if err != nil {
		return parsedEnvInputs{}, err
	}

	failOnChanges, err := parseFailOnChanges(raw.failOnChangesRaw)
	if err != nil {
		return parsedEnvInputs{}, err
	}

	err = validateStoreVersion(raw.storeVersion)
	if err != nil {
		return parsedEnvInputs{}, err
	}

	targetFolder := DefaultTargetFolder
	if raw.targetFolderRaw != "" {
		targetFolder = raw.targetFolderRaw
	}

	normalizedTarget, err := pathutil.ValidateTargetFolder(targetFolder, raw.workspace)
	if err != nil {
		return parsedEnvInputs{}, fmt.Errorf("validate target folder: %w", err)
	}

	rootTaskfile, err := resolveRootTaskfile(raw.rootTaskfileRaw, normalizedTarget, raw.workspace)
	if err != nil {
		return parsedEnvInputs{}, err
	}

	return parsedEnvInputs{
		tasks:            tasks,
		jsRuntime:        jsRuntime,
		packageManager:   packageManager,
		versionManager:   versionManager,
		includesDoc:      includesDoc,
		syncRoot:         syncRoot,
		failOnChanges:    failOnChanges,
		normalizedTarget: normalizedTarget,
		rootTaskfile:     rootTaskfile,
	}, nil
}

// resolveRootTaskfile determines where the generated aggregator Taskfile is
// written. When unset it defaults to <targetFolder>/Taskfile.yml; otherwise the
// caller-provided workspace-relative path is validated and must be a YAML file.
func resolveRootTaskfile(raw, targetFolder, workspace string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pathutil.JoinRelative(targetFolder, "Taskfile.yml"), nil
	}

	normalized, err := pathutil.ValidateRelativePath(workspace, raw)
	if err != nil {
		return "", fmt.Errorf("validate root-taskfile: %w", err)
	}

	if !strings.HasSuffix(normalized, ".yml") && !strings.HasSuffix(normalized, ".yaml") {
		return "", &ValidationError{
			Field:   "root-taskfile",
			Message: fmt.Sprintf("must be a .yml or .yaml file path, got %q", raw),
		}
	}

	return normalized, nil
}

type rawEnvConfig struct {
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

func loadRawEnv() rawEnvConfig {
	token := actionInput("github-token")
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}

	return rawEnvConfig{
		tasksRaw:         actionInput("tasks"),
		jsRaw:            actionInput("js"),
		includesDocRaw:   actionInput("includes-doc"),
		syncRootRaw:      actionInput("sync-root"),
		failOnChangesRaw: actionInput("fail-on-changes"),
		storeVersion:     actionInput("store-version"),
		targetFolderRaw:  actionInput("target-folder"),
		rootTaskfileRaw:  actionInput("root-taskfile"),
		token:            token,
		workspace:        os.Getenv("GITHUB_WORKSPACE"),
		repository:       os.Getenv("GITHUB_REPOSITORY"),
		githubOutput:     os.Getenv("GITHUB_OUTPUT"),
		githubRef:        os.Getenv("GITHUB_REF"),
		githubBaseRef:    os.Getenv("GITHUB_BASE_REF"),
	}
}

// resolveBaseBranch returns the branch the workflow is operating against.
// Pull request events expose their target branch through GITHUB_BASE_REF;
// push, schedule, and workflow_dispatch events use a refs/heads/... GITHUB_REF.
func resolveBaseBranch(githubBaseRef, githubRef string) string {
	if branch := strings.TrimSpace(githubBaseRef); branch != "" {
		return branch
	}

	const branchRefPrefix = "refs/heads/"
	if branch, ok := strings.CutPrefix(strings.TrimSpace(githubRef), branchRefPrefix); ok {
		return strings.TrimSpace(branch)
	}

	return ""
}

func validateRuntimeEnv(workspace, token string) error {
	if workspace == "" {
		return &ValidationError{Field: "GITHUB_WORKSPACE", Message: "is required"}
	}

	if token == "" {
		return missingActionInput("github-token")
	}

	return nil
}

func jsSettingsFromConfig(jsCfg *jsConfig) (JSRuntime, PackageManager, VersionManager) {
	if jsCfg == nil {
		return "", "", ""
	}

	return jsCfg.Runtime, jsCfg.NodePackageManager, jsCfg.NodeVersionManager
}

func parseTasks(raw string) ([]string, error) {
	raw = strings.ReplaceAll(raw, ",", "\n")
	lines := strings.Split(raw, "\n")
	seen := make(map[string]struct{})

	var tasks []string

	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}

		err := pathutil.ValidateTaskName(name)
		if err != nil {
			return nil, fmt.Errorf("validate task name: %w", err)
		}

		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		tasks = append(tasks, name)
	}

	if len(tasks) == 0 {
		return nil, &ValidationError{Field: "tasks", Message: "at least one task is required"}
	}

	return tasks, nil
}

func parseIncludesDoc(raw string) (bool, error) {
	return parseBoolInput("includes-doc", raw, true)
}

func parseSyncRoot(raw string) (bool, error) {
	return parseBoolInput("sync-root", raw, true)
}

func parseFailOnChanges(raw string) (bool, error) {
	return parseBoolInput("fail-on-changes", raw, false)
}

func parseBoolInput(field, raw string, defaultValue bool) (bool, error) {
	if raw == "" {
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

func validateStoreVersion(version string) error {
	if version == "" {
		return nil
	}

	if unsafeStoreVersion.MatchString(version) {
		return &ValidationError{
			Field:   "store-version",
			Message: fmt.Sprintf("unsafe revision expression %q", version),
		}
	}

	return nil
}

func computeConfigurationHash(payload hashPayload) (string, string) {
	data, err := json.Marshal(map[string]any{
		"tasks":                payload.Tasks,
		"node_package_manager": payload.NodePackageManager,
		"node_version_manager": payload.NodeVersionManager,
		"target_folder":        payload.TargetFolder,
		"store_version":        payload.StoreVersion,
		"includes_doc":         payload.IncludesDoc,
		"sync_root":            payload.SyncRoot,
	})
	if err != nil {
		data = []byte("{}")
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	return hash, "taskotter/sync-" + hash[:12]
}

// LockFilePath returns the workspace-relative path to the managed lock file.
func (c *Config) LockFilePath() string {
	return pathutil.JoinRelative(c.TargetFolder, ".taskotter-lock.yml")
}

// MetadataPath returns the workspace-relative path to TaskOtter metadata.
func (c *Config) MetadataPath() string {
	return pathutil.JoinRelative(c.TargetFolder, ".taskotter/metadata.yml")
}
