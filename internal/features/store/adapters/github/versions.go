// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"context"
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"golang.org/x/mod/semver"
)

// LatestStableVersion returns the highest stable version tag, or empty when none exist.
func (client *Client) LatestStableVersion(ctx context.Context) (string, error) {
	var latest string

	for page := consts.IndexOne; ; page++ {
		tags, err := listTags(ctx, client, page)
		if err != nil {
			return "", fmt.Errorf("find latest stable store version: %w", err)
		}

		latest = latestStablePage(latest, tags)

		if len(tags) < tagsPerPage {
			return latest, nil
		}
	}
}

func latestStablePage(latest string, tags []tagPayload) string {
	for i := range tags {
		latest = latestStableTag(latest, tags[i].Name)
	}

	return latest
}

func latestStableTag(latest, candidate string) string {
	version := domain.SemanticVersion(candidate)

	if version == "" || semver.Prerelease(version) != "" {
		return latest
	}

	return newerTag(latest, candidate)
}

func listTags(ctx context.Context, client *Client, page int) ([]tagPayload, error) {
	var tags []tagPayload

	reqPath := fmt.Sprintf(
		"/repos/%s/%s/tags?per_page=%d&page=%d",
		storeOwner,
		storeRepo,
		tagsPerPage,
		page,
	)

	err := getJSON(ctx, client, &getJSONArgs{reqPath: reqPath, payload: &tags})
	if err != nil {
		return nil, fmt.Errorf("list store tags page %d: %w", page, err)
	}

	return tags, nil
}

func newerTag(latest, candidate string) string {
	comparison := semver.Compare(domain.SemanticVersion(candidate), domain.SemanticVersion(latest))

	if comparison > consts.IndexZero || (comparison == consts.IndexZero && candidate < latest) {
		return candidate
	}

	return latest
}
