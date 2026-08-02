// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config

import (
	"fmt"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

const (
	fieldJSPackageManager = "js.package-manager"
	fieldJSVersionManager = "js.version-manager"
)

// JSRuntime selects the JavaScript runtime for Node-oriented task resolution.
type JSRuntime string

const (
	// JSRuntimeBun selects Bun as the JS runtime.
	JSRuntimeBun JSRuntime = "bun"
	// JSRuntimeNodeJS selects Node.js as the JS runtime.
	JSRuntimeNodeJS JSRuntime = "nodejs"
)

type jsInput struct {
	Runtime        string
	PackageManager string
	VersionManager string
}

type jsConfig struct {
	Runtime            JSRuntime
	NodePackageManager PackageManager
	NodeVersionManager VersionManager
}

func parseJS(raw string) (*jsConfig, error) {
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return &jsConfig{
			Runtime:            "",
			NodePackageManager: "",
			NodeVersionManager: "",
		}, nil
	}

	yamlInput, err := parseJSInput(raw)
	if err != nil {
		return nil, &ValidationError{
			Field:   "js",
			Message: fmt.Sprintf("invalid YAML: %v", err),
		}
	}

	runtime := strings.TrimSpace(yamlInput.Runtime)

	if runtime == "" {
		runtime = string(JSRuntimeNodeJS)
	}

	switch JSRuntime(runtime) {
	case JSRuntimeBun:
		return parseJSBun(yamlInput)
	case JSRuntimeNodeJS:
		return parseJSNodeJS(yamlInput)
	default:
		return nil, &ValidationError{
			Field:   "js.runtime",
			Message: fmt.Sprintf("invalid value %q: allowed values are bun or nodejs", runtime),
		}
	}
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

func parseJSBun(yamlInput jsInput) (*jsConfig, error) {
	if strings.TrimSpace(yamlInput.PackageManager) != "" {
		return nil, &ValidationError{
			Field:   fieldJSPackageManager,
			Message: "is only valid when js.runtime is nodejs",
		}
	}

	if strings.TrimSpace(yamlInput.VersionManager) != "" {
		return nil, &ValidationError{
			Field:   fieldJSVersionManager,
			Message: "is only valid when js.runtime is nodejs",
		}
	}

	return &jsConfig{
		Runtime:            JSRuntimeBun,
		NodePackageManager: PMBun,
		NodeVersionManager: "",
	}, nil
}

func parseJSNodeJS(yamlInput jsInput) (*jsConfig, error) {
	packageManagerRaw := strings.TrimSpace(yamlInput.PackageManager)

	if packageManagerRaw == "" {
		packageManagerRaw = string(PMNPM)
	}

	versionManagerRaw := strings.TrimSpace(yamlInput.VersionManager)

	if versionManagerRaw == "" {
		versionManagerRaw = string(VMFnm)
	}

	packageManager, err := parseNodePackageManager(packageManagerRaw)
	if err != nil {
		return nil, err
	}

	if packageManager == PMBun {
		return nil, &ValidationError{
			Field:   fieldJSPackageManager,
			Message: `use js.runtime "bun" instead of package-manager "bun"`,
		}
	}

	versionManager, err := parseNodeVersionManager(versionManagerRaw)
	if err != nil {
		return nil, err
	}

	return &jsConfig{
		Runtime:            JSRuntimeNodeJS,
		NodePackageManager: packageManager,
		NodeVersionManager: versionManager,
	}, nil
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
