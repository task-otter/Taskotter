// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config

type (

	// FieldError is a configuration validation error with a field name.
	FieldError interface {
		error
		FieldName() string
	}

	// ValidationError reports invalid action input values.
	//
	// Field is the input name; Message explains the rejection.
	ValidationError struct {
		Field   string
		Message string
	}

	// PackageManager selects the Node package manager for JS task resolution.
	PackageManager = string

	// Config holds validated TaskOtter action inputs and derived sync metadata.
	Config = struct {
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

	hashPayload = struct {
		NodePackageManager string
		TargetFolder       string
		StoreVersion       string
		Tasks              []string
		IncludesDoc        bool
		SyncRoot           bool
	}

	parsedEnvInputs = struct {
		jsRuntime        JSRuntime
		packageManager   PackageManager
		normalizedTarget string
		rootTaskfile     string
		tasks            []string
		includesDoc      bool
		syncRoot         bool
		failOnChanges    bool
	}

	tasksAndJSSettings = struct {
		jsRuntime      JSRuntime
		packageManager PackageManager
		tasks          []string
	}

	jsSettings = struct {
		jsRuntime      JSRuntime
		packageManager PackageManager
	}

	toggleFlags = struct {
		includesDoc   bool
		syncRoot      bool
		failOnChanges bool
	}

	targetPaths = struct {
		normalizedTarget string
		rootTaskfile     string
	}

	rawEnvConfig = struct {
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

	assembleConfigInput = struct {
		Raw    *rawEnvConfig
		Parsed *parsedEnvInputs
		Hash   string
		Branch string
	}

	mergeParsedArgs = struct {
		tasks *tasksAndJSSettings
		flags *toggleFlags
		paths *targetPaths
	}

	// JSRuntime selects the JavaScript runtime for Node-oriented task resolution.
	JSRuntime = string

	jsInput = struct {
		Runtime        string
		PackageManager string
		VersionManager string
	}

	jsConfig = struct {
		Runtime            JSRuntime
		NodePackageManager PackageManager
	}
)
