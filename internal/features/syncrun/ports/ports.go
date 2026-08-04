// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package ports defines syncrun composition-root dependencies.
package ports

import (
	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	prports "github.com/task-otter/Taskotter/internal/features/pr/ports"
	storeports "github.com/task-otter/Taskotter/internal/features/store/ports"
)

type (
	// StoreClient resolves store refs and downloads snapshots for the sync pipeline.
	StoreClient = storeports.SnapshotClient

	// PRClient creates and updates TaskOtter sync pull requests.
	PRClient = prports.PRClient

	// Brancher manages local branch refs.
	Brancher = gitports.Brancher

	// Indexer inspects and stages the working tree.
	Indexer = gitports.Indexer

	// Publisher pushes branches to origin.
	Publisher = gitports.Publisher
)
