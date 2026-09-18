// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package env

import (
	"os"
	"strings"
)

const emptyValue = ""

// Input reads INPUT_<NAME>, accepting both hyphen-preserving and underscore
// spellings used by different GitHub Actions runners.
func Input(name string) string {
	upper := strings.ToUpper(name)
	keys := [...]string{
		"INPUT_" + upper,
		"INPUT_" + strings.ReplaceAll(upper, "-", "_"),
	}

	for index := range keys {
		if value := strings.TrimSpace(os.Getenv(keys[index])); value != emptyValue {
			return value
		}
	}

	return emptyValue
}

// Token reads the action token and falls back to GITHUB_TOKEN.
func Token() string {
	if token := Input("github-token"); token != emptyValue {
		return token
	}

	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
}
