// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"context"

	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"golang.org/x/mod/semver"
)

func warnNewerStoreVersion(ctx context.Context, deps *Deps, requested string) {
	latest, err := deps.StoreClient.LatestStableVersion(ctx)
	if err != nil {
		deps.Logger.Warningf(
			"Could not check latest stable store version: %v. Continuing with %s.",
			err, requested,
		)

		return
	}

	version := storedomain.SemanticVersion(requested)

	if semver.Compare(version, storedomain.SemanticVersion(latest)) < consts.IndexZero {
		deps.Logger.Warningf(
			"Store version %s is outdated; latest stable version is %s. "+
				"Update store-version to %s. Continuing with %s.",
			requested, latest, latest, requested,
		)
	}
}

func warnOutdatedStoreVersion(ctx context.Context, deps *Deps, requested string) {
	if storedomain.SemanticVersion(requested) == consts.Empty {
		deps.Logger.Printf(
			"Store version %s is not a semantic version; freshness cannot be compared.",
			requested,
		)

		return
	}

	warnNewerStoreVersion(ctx, deps, requested)
}
