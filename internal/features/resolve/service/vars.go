// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
)

var (
	errModuleNotDefined = errors.New("module is not defined in .deps.yml")

	_ DepsResolver = transitiveResolver{}

	errEmptyNormalizedName = errors.New("normalized name is empty")

	errInvalidPackageManager = errors.New("invalid package manager")
)
