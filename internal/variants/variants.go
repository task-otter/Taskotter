// Package variants resolves Node task source module names from runtime configuration.
package variants

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/config"
)

var (
	errVersionManagerRequired = errors.New("js.version-manager required for package manager")
	errInvalidPackageManager  = errors.New("invalid package manager")
)

func nodeToolSuffixes() []string {
	return []string{
		"node/fnm/npm", "node/fnm/yarn", "node/fnm/pnpm",
		"node/nvm/npm", "node/nvm/yarn", "node/nvm/pnpm",
		"bun",
	}
}

func stripSuffixes() []string {
	return []string{
		"/node/fnm/npm",
		"/node/fnm/yarn",
		"/node/fnm/pnpm",
		"/node/nvm/npm",
		"/node/nvm/yarn",
		"/node/nvm/pnpm",
		"/bun",
		"/fnm",
		"/nvm",
		"-npm-fnm",
		"-npm-nvm",
		"-yarn-fnm",
		"-yarn-nvm",
		"-pnpm-fnm",
		"-pnpm-nvm",
		"-bun",
		"-fnm",
		"-nvm",
	}
}

// IsNodeToolVariant reports whether moduleName is a Node variant of logicalTask.
func IsNodeToolVariant(moduleName, logicalTask string) bool {
	prefix := logicalTask + "/"
	if len(moduleName) <= len(prefix) {
		return false
	}

	if moduleName[:len(prefix)] != prefix {
		return false
	}

	suffix := strings.TrimPrefix(moduleName[len(prefix):], "/")

	return slices.Contains(nodeToolSuffixes(), suffix)
}

// BuildSourceModule constructs the store module name for a logical task and JS configuration.
func BuildSourceModule(
	task string,
	packageManager config.PackageManager,
	versionManager config.VersionManager,
) (string, error) {
	switch packageManager {
	case config.PMBun:
		return path.Join(task, string(packageManager)), nil
	case config.PMNPM, config.PMYarn, config.PMPnpm:
		if versionManager == "" {
			return "", fmt.Errorf("%w %q", errVersionManagerRequired, packageManager)
		}

		return path.Join(task, "node", string(versionManager), string(packageManager)), nil
	default:
		return "", fmt.Errorf("%w %q", errInvalidPackageManager, packageManager)
	}
}

// StripOneSuffix removes one known suffix from the end of name.
func StripOneSuffix(name string) (string, bool) {
	for _, suffix := range stripSuffixes() {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			stripped := name[:len(name)-len(suffix)]
			if stripped == "" {
				continue
			}

			return stripped, true
		}
	}

	return name, false
}
