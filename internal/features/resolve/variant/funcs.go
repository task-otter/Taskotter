// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package variant

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

func nodeToolSuffixes() []string {
	return []string{
		"node/npm", "node/yarn", "node/pnpm",
		"bun",
	}
}

func stripSuffixes() []string {
	return []string{
		"/node/npm",
		"/node/yarn",
		"/node/pnpm",
		"/bun",
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
func BuildSourceModule(task string, packageManager pkgMgr) (string, error) {
	switch packageManager {
	case config.JSRuntimeBun:
		return path.Join(task, packageManager), nil
	case config.PMNPM, config.PMYarn, config.PMPnpm:
		return path.Join(task, "node", packageManager), nil
	default:
		return consts.Empty, fmt.Errorf(fmtWrapQuoted, errInvalidPackageManager, packageManager)
	}
}

// StripOneSuffix removes one known suffix from the end of name.
func StripOneSuffix(name string) (string, bool) {
	suffixes := stripSuffixes()

	for i := range suffixes {
		suffix := suffixes[i]

		if !hasSuffixLongerThan(name, suffix) {
			continue
		}

		return name[:len(name)-len(suffix)], true
	}

	return name, false
}

func hasSuffixLongerThan(name, suffix string) bool {
	return len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix
}
