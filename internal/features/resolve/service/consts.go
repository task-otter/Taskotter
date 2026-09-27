// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

const (
	errFmtVisitModule = "visit module %q: %w"

	maxCloseMatches      = 5
	scoreExactMatch      = 1000
	scorePrefixMatchBase = 500
	scoreIdenticalString = 100

	fmtWrapQuoted = "%w %q"
)
