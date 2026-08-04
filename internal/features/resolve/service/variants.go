// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

type (
	pkgMgr = config.PackageManager
	verMgr = config.VersionManager
)

const (
	fmtWrapQuoted = "%w %q"
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
func BuildSourceModule(task string, packageManager pkgMgr, versionManager verMgr) (string, error) {
	switch packageManager {
	case config.PackageManager(config.JSRuntimeBun):
		return path.Join(task, string(packageManager)), nil
	case config.PMNPM, config.PMYarn, config.PMPnpm:
		module, err := buildNodePackageModule(task, packageManager, versionManager)
		if err != nil {
			return consts.Empty, fmt.Errorf("build node package module: %w", err)
		}

		return module, nil
	default:
		return consts.Empty, fmt.Errorf(fmtWrapQuoted, errInvalidPackageManager, packageManager)
	}
}

func buildNodePackageModule(task string, pkg pkgMgr, ver verMgr) (string, error) {
	if ver == consts.Empty {
		return consts.Empty, fmt.Errorf(
			fmtWrapQuoted,
			errVersionManagerRequired,
			pkg,
		)
	}

	return path.Join(task, "node", string(ver), string(pkg)), nil
}

// StripOneSuffix removes one known suffix from the end of name.
func StripOneSuffix(name string) (string, bool) {
	suffixes := stripSuffixes()

	for i := range suffixes {
		suffix := suffixes[i]

		if !hasSuffixLongerThan(name, suffix) {
			continue
		}

		stripped := name[:len(name)-len(suffix)]

		if stripped == consts.Empty {
			continue
		}

		return stripped, true
	}

	return name, false
}

func hasSuffixLongerThan(name, suffix string) bool {
	return len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix
}
