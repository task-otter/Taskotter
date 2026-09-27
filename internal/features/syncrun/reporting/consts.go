// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package reporting

const (
	syncRequiredErrorSuffix = " Merge the sync pull request to update taskfiles, then re-run this workflow.\n"
	syncRequiredNotice      = "::notice title=What happened::TaskOtter compared managed files " +
		"with the store and found drift. This job fails intentionally until the sync PR is merged.\n"
	syncUpToDateNotice = "::notice title=TaskOtter sync up to date::Managed taskfiles " +
		"match the store. No sync pull request was created.\n"
)
