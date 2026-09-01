// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config

import (
	"fmt"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
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

func jsRuntimeParsers() map[JSRuntime]func(*jsInput) (*jsConfig, error) {
	return map[JSRuntime]func(*jsInput) (*jsConfig, error){
		JSRuntimeBun:    parseJSBun,
		JSRuntimeNodeJS: parseJSNodeJS,
	}
}

func defaultedJSRuntime(rawRuntime string) string {
	runtime := strings.TrimSpace(rawRuntime)

	if runtime == consts.Empty {
		runtime = string(JSRuntimeNodeJS)
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
		return nil, err
	}

	runtime := defaultedJSRuntime(yamlInput.Runtime)

	parser, ok := jsRuntimeParsers()[JSRuntime(runtime)]

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
		NodePackageManager: PackageManager(JSRuntimeBun),
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
	packageManagerRaw := defaultedRaw(yamlInput.PackageManager, string(PMNPM))

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
		return PackageManager(raw), nil
	default:
		return consts.Empty, &ValidationError{
			Field:   fieldJSPackageManager,
			Message: fmt.Sprintf("invalid value %q: allowed values are npm, yarn, or pnpm", raw),
		}
	}
}

func validatePackageManager(raw string) (PackageManager, error) {
	packageManager, err := parseNodePackageManager(raw)
	if err != nil {
		return consts.Empty, fmt.Errorf("parse node package manager: %w", err)
	}

	if packageManager == PackageManager(JSRuntimeBun) {
		return consts.Empty, &ValidationError{
			Field:   fieldJSPackageManager,
			Message: `use js.runtime "bun" instead of package-manager "bun"`,
		}
	}

	return packageManager, nil
}
