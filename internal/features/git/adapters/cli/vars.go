// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"errors"
	"regexp"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
)

var (
	_ gitports.Workspace = (*Client)(nil)
	_ gitports.Brancher  = (*Client)(nil)
	_ gitports.Indexer   = (*Client)(nil)
	_ gitports.Publisher = (*Client)(nil)

	//nolint:gochecknoglobals // seam so tests can stub the git command
	gitBinary = "git"

	errOriginHEADNotAvailable = errors.New("origin HEAD not available")

	errNoRemoteBranchAtOriginHEAD = errors.New("no remote branch at origin HEAD commit")

	errHEADBranchNotFound = errors.New("HEAD branch not found in remote show output")

	errDefaultBranchDetectionFailed = errors.New(
		"detect default branch: none of the detection methods succeeded",
	)

	errBranchNotOwned = errors.New("branch exists but is not owned by TaskOtter")

	errInvalidGitRef = errors.New("invalid git ref")

	errInvalidStagePath = errors.New("invalid stage path")

	errInvalidRepository = errors.New("invalid repository")

	gitRefPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

	repoSegmentPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
)
