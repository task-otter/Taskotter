// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package env

import (
	"os"
	"strings"
)

// Input reads INPUT_<NAME>, accepting both hyphen-preserving and underscore
// spellings used by different GitHub Actions runners.
func Input(name string) string {
	upper := strings.ToUpper(name)
	keys := [...]string{
		"INPUT_" + upper,
		"INPUT_" + strings.ReplaceAll(upper, "-", "_"),
	}

	for index := range keys {
		if value := strings.TrimSpace(os.Getenv(keys[index])); value != "" {
			return value
		}
	}

	return ""
}

// Token reads the action token and falls back to GITHUB_TOKEN.
func Token() string {
	if token := Input("github-token"); token != "" {
		return token
	}

	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}
