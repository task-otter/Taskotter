// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package safety

import (
	"path/filepath"
	"strings"
)

const parentPath = ".."

// Escapes reports whether a relative path leaves its extraction root.
func Escapes(rel string) bool {
	return rel == parentPath || strings.HasPrefix(rel, parentPath+string(filepath.Separator))
}

// IsSafeTarPath rejects absolute, parent-traversing, and platform-specific
// backslash paths from tar headers.
func IsSafeTarPath(name string) bool {
	if filepath.IsAbs(name) {
		return false
	}

	clean := filepath.Clean(name)

	return !strings.HasPrefix(clean, "..") && !strings.Contains(name, "\\")
}
