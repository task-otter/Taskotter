// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"time"
)

const (
	storeOwner = "task-otter"

	storeRepo = "store"

	gitObjectTypeTag = "tag"

	httpClientTimeout = 60 * time.Second

	defaultBaseURL = "https://api.github.com"

	fmtExtractAndLoadErr = "extract and load: %w"

	fmtArchiveDownloadStatusErr = "archive download status: %w"

	fmtCloseResponseBodyErr = "close response body: %w"

	fmtDrainResponseBodyErr = "drain response body: %w"

	fmtApplyResolvedRefErr = "%s: %w"
)
