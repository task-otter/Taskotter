// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"context"
	"net/http"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
)

type (
	// RefInfo is store ref metadata from the domain package.
	RefInfo = domain.RefInfo
	// Snapshot is an extracted store tree from the domain package.
	Snapshot = domain.Snapshot

	// HTTPDoer performs HTTP requests for the store client.
	HTTPDoer = interface {
		Do(req *http.Request) (*http.Response, error)
	}

	httpRsp = *http.Response

	// Client resolves store refs and downloads store archives from GitHub.
	decodeTagArgs = struct {
		rsp httpRsp
		val string
	}

	getJSONArgs = struct {
		payload any
		reqPath string
	}

	clientFns = struct {
		do      func(*http.Request) (*http.Response, error)
		token   string
		baseURL string
	}

	// Client resolves store refs and downloads store archives from GitHub.
	Client struct {
		fns clientFns
	}

	// tagRefPayload is the GitHub API response shape for a tag ref lookup.
	tagRefPayload = struct {
		Object tagRefObject `json:"object"`
	}

	tagRefObject = struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	}

	annotatedTagPayload = struct {
		Object annotatedTagObject `json:"object"`
	}

	annotatedTagObject = struct {
		SHA string `json:"sha"`
	}

	branchHeadPayload = struct {
		SHA string `json:"sha"`
	}

	extractedStore = struct {
		catalog map[string]struct{}
		deps    map[string][]string
		root    string
	}

	versionRefRequest = struct {
		info             *RefInfo
		requestedVersion string
		defaultBranch    string
	}

	resolvedRefRequest = struct {
		info      *RefInfo
		resolve   func(context.Context) (string, error)
		sourceRef string
		wrapMsg   string
	}
)
