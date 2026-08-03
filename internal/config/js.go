// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config

import (
	"fmt"
	"strings"

	"github.com/task-otter/Taskotter/internal/consts"
	yaml "go.yaml.in/yaml/v3"
)

type (

	// JSRuntime selects the JavaScript runtime for Node-oriented task resolution.
	JSRuntime string

	jsInput struct {
		Runtime        string
		PackageManager string
		VersionManager string
	}

	jsConfig struct {
		Runtime            JSRuntime
		NodePackageManager PackageManager
		NodeVersionManager VersionManager
	}
)

const (
	fieldJSPackageManager = "js.package-manager"
	fieldJSVersionManager = "js.version-manager"

	// JSRuntimeBun selects Bun as the JS runtime.
	JSRuntimeBun JSRuntime = JSRuntime(consts.Bun)

	// JSRuntimeNodeJS selects Node.js as the JS runtime.
	JSRuntimeNodeJS JSRuntime = "nodejs"
)

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

func emptyJSConfig() *jsConfig {
	return &jsConfig{
		Runtime:            consts.Empty,
		NodePackageManager: consts.Empty,
		NodeVersionManager: consts.Empty,
	}
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

func dispatchJSRuntime(yamlInput *jsInput) (*jsConfig, error) {
	runtime := defaultedJSRuntime(yamlInput.Runtime)

	switch JSRuntime(runtime) {
	case JSRuntimeBun:
		jsCfg, err := parseJSBun(yamlInput)
		if err != nil {
			return nil, fmt.Errorf("parse bun config: %w", err)
		}

		return jsCfg, nil
	case JSRuntimeNodeJS:
		jsCfg, err := parseJSNodeJS(yamlInput)
		if err != nil {
			return nil, fmt.Errorf("parse nodejs config: %w", err)
		}

		return jsCfg, nil
	default:
		return nil, &ValidationError{
			Field:   "js.runtime",
			Message: fmt.Sprintf("invalid value %q: allowed values are bun or nodejs", runtime),
		}
	}
}

func defaultedJSRuntime(rawRuntime string) string {
	runtime := strings.TrimSpace(rawRuntime)

	if runtime == consts.Empty {
		runtime = string(JSRuntimeNodeJS)
	}

	return runtime
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

func parseJSBun(yamlInput *jsInput) (*jsConfig, error) {
	if strings.TrimSpace(yamlInput.PackageManager) != consts.Empty {
		return nil, &ValidationError{
			Field:   fieldJSPackageManager,
			Message: consts.JSValidOnlyForNodejs,
		}
	}

	if strings.TrimSpace(yamlInput.VersionManager) != consts.Empty {
		return nil, &ValidationError{
			Field:   fieldJSVersionManager,
			Message: consts.JSValidOnlyForNodejs,
		}
	}

	return &jsConfig{
		Runtime:            JSRuntimeBun,
		NodePackageManager: PMBun,
		NodeVersionManager: consts.Empty,
	}, nil
}

func parseJSNodeJS(yamlInput *jsInput) (*jsConfig, error) {
	packageManagerRaw := defaultedRaw(yamlInput.PackageManager, string(PMNPM))
	versionManagerRaw := defaultedRaw(yamlInput.VersionManager, string(VMFnm))

	packageManager, err := validatePackageManager(packageManagerRaw)
	if err != nil {
		return nil, fmt.Errorf("validate package manager: %w", err)
	}

	versionManager, err := parseNodeVersionManager(versionManagerRaw)
	if err != nil {
		return nil, fmt.Errorf("parse node version manager: %w", err)
	}

	return &jsConfig{
		Runtime:            JSRuntimeNodeJS,
		NodePackageManager: packageManager,
		NodeVersionManager: versionManager,
	}, nil
}

func defaultedRaw(raw, fallback string) string {
	trimmed := strings.TrimSpace(raw)

	if trimmed == consts.Empty {
		return fallback
	}

	return trimmed
}

func validatePackageManager(raw string) (PackageManager, error) {
	packageManager, err := parseNodePackageManager(raw)
	if err != nil {
		return "", fmt.Errorf("parse node package manager: %w", err)
	}

	if packageManager == PMBun {
		return "", &ValidationError{
			Field:   fieldJSPackageManager,
			Message: `use js.runtime "bun" instead of package-manager "bun"`,
		}
	}

	return packageManager, nil
}

func parseNodePackageManager(raw string) (PackageManager, error) {
	switch raw {
	case "npm", "yarn", "pnpm":
		return PackageManager(raw), nil
	default:
		return "", &ValidationError{
			Field:   fieldJSPackageManager,
			Message: fmt.Sprintf("invalid value %q: allowed values are npm, yarn, or pnpm", raw),
		}
	}
}

func parseNodeVersionManager(raw string) (VersionManager, error) {
	switch raw {
	case "fnm", "nvm":
		return VersionManager(raw), nil
	default:
		return "", &ValidationError{
			Field:   fieldJSVersionManager,
			Message: fmt.Sprintf("invalid value %q: allowed values are fnm or nvm", raw),
		}
	}
}
