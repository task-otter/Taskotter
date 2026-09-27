// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package config

import (
	"regexp"
)

var unsafeStoreVersion = regexp.MustCompile(`(?i)(^refs/|\.\./|/|\\|\^|~|\^{commit})`)
