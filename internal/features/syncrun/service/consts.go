// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

const (
	fmtArrow = "%s -> %s"

	fmtGroupErr = "%s: %w"

	fmtTargetFolder = "Target folder: %s"

	groupSummary = "Summary"

	errFmtBuildSyncPlan = "build sync plan: %w"

	errFmtCheckUnrelatedChanges = "check unrelated changes: %w"

	errFmtFindOpenPullRequest = "find open pull request: %w"

	errFmtResolveStoreRef = "resolve store ref: %w"

	errFmtRun = "run: %w"

	errFmtResolveDependencies = "resolve dependencies: %w"

	errFmtResolveRequestedModules = "resolve requested modules: %w"

	errFmtResolveTransitiveDeps = "resolve transitive dependencies: %w"

	fmtRunGroupedErr = "run grouped: %w"

	syncRequiredErrorSuffix = " Merge the sync pull request to update taskfiles, then re-run this workflow.\n"

	jsonIndent = "  "

	syncRequiredNotice = "::notice title=What happened::TaskOtter compared managed files " +
		"with the store and found drift. This job fails intentionally until the sync PR is merged.\n"

	syncUpToDateNotice = "::notice title=TaskOtter sync up to date::Managed taskfiles " +
		"match the store. No sync pull request was created.\n"
)
