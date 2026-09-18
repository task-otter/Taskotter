// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"errors"
	"regexp"
)

const (
	gitBinary                = "git"
	errCheckBranchExists     = "check branch exists: %w"
	errReadLastCommitMessage = "read last commit message: %w"
	errStagePaths            = "stage paths: %w"
	errRunGitOutput          = "run git output: %w"
)

var (
	errOriginHEADNotAvailable = errors.New("origin HEAD not available")

	errNoRemoteBranchAtOriginHEAD = errors.New("no remote branch at origin HEAD commit")

	errHEADBranchNotFound = errors.New("HEAD branch not found in remote show output")

	errDefaultBranchDetectionFailed = errors.New(
		"detect default branch: none of the detection methods succeeded",
	)

	errBranchNotOwned = errors.New("branch exists but is not owned by TaskOtter")

	errInvalidStagePath = errors.New("invalid stage path")

	errInvalidRepository = errors.New("invalid repository")

	repoSegmentPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)
