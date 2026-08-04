// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package ports defines store feature outbound dependencies.
package ports

import (
	"context"
	"net/http"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	// HTTPDoer performs HTTP requests for the store client.
	HTTPDoer interface {
		Do(req *http.Request) (*http.Response, error)
	}

	// SnapshotClient resolves store refs and downloads snapshots.
	SnapshotClient interface {
		ResolveRef(ctx context.Context, requestedVersion string) (domain.RefInfo, error)
		DownloadSnapshot(ctx context.Context, ref *domain.RefInfo) (*domain.Snapshot, error)
	}
)
