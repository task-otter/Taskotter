// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

const (

	// SyncCommitMessage is the commit message TaskOtter uses for sync branches.
	SyncCommitMessage = "chore(taskotter): sync taskfiles"

	commitUserName = "Taskotter"

	commitUserEmail = "taskotter@users.noreply.github.com"

	gitStatusPathOffset = 3

	gitRemote = "remote"

	gitVerifyFlag = "--verify"

	gitConfigFlag = "-c"

	fmtGitCmdErr = "git %s: %w: %s"

	argSep = " "

	errMustNotStartWithHyphen = "%w: must not start with '-'"

	fmtValidateGitRefErr = "validate git ref: %w"

	gitCheckout = "checkout"

	fmtCheckoutBranchErr = "checkout branch: %w"

	gitPush = "push"

	fmtGitPushErr = "git push: %w"

	maxGitRefLen = 255

	originHEADShortRef = "origin/HEAD"

	httpExtraHeaderKey = "http.https://github.com/.extraheader"

	gitConfigLocal = "--local"

	gitConfigUnsetAll = "--unset-all"

	//nolint:gosec // G101: GitHub x-access-token username scheme, not a secret
	originAccessURLFmt = "https://x-access-token:%s@github.com/%s.git"
)
