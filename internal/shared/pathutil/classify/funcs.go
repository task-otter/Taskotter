// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package classify

import (
	"strings"
)

// NormalizeSlashes converts platform separators to forward slashes.
func NormalizeSlashes(path string) string { return strings.ReplaceAll(path, "\\", separator) }

// IsDocPath reports whether path is a README or a file below a docs directory.
func IsDocPath(path string) bool {
	path = NormalizeSlashes(path)

	return path == readme || strings.HasPrefix(path, "docs"+separator) ||
		strings.Contains(path, separator+"docs"+separator)
}

// IsModuleMetadataPath reports whether path is module metadata.
func IsModuleMetadataPath(path string) bool { return NormalizeSlashes(path) == metadata }

// IsTestPath reports whether the file basename contains the Go test suffix.
func IsTestPath(path string) bool {
	path = NormalizeSlashes(path)

	base := path

	if index := strings.LastIndex(path, separator); index >= 0 {
		base = path[index+1:]
	}

	return strings.Contains(base, "_test.")
}

// HasFolderPrefix reports whether path equals folder or is below it.
func HasFolderPrefix(path, folder string) bool {
	path = NormalizeSlashes(path)
	folder = NormalizeSlashes(folder)

	return path == folder || strings.HasPrefix(path, folder+separator)
}
