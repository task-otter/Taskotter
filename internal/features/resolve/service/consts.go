// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

const (
	jsonKeySourceModule      = "source_module"
	jsonKeyDestinationModule = "destination_module"
	jsonKeyPath              = "path"
	resolveApp               = "app"
	resolveLib               = "lib"
	resolveTask              = "task"
	fmtResolve               = "Resolve() = %#v"
	errFmtVisitModule        = "visit module %q: %w"

	maxCloseMatches      = 5
	scoreExactMatch      = 1000
	scorePrefixMatchBase = 500
	scoreIdenticalString = 100

	fmtWrapQuoted = "%w %q"
)
