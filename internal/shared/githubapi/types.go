// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package githubapi

import (
	"net/http"
	"net/url"
)

type (

	// Client wraps GitHub pull request API calls.
	doer interface {
		Do(req *http.Request) (*http.Response, error)
	}

	clientFns = struct {
		do      func(*http.Request) (*http.Response, error)
		baseURL *url.URL
	}

	// Client wraps GitHub pull request API calls.
	Client struct {
		fns clientFns
	}

	// PullRequest is a minimal pull request view.
	PullRequest = struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}

	// ListOpenPROptions selects open pull requests for a head/base pair.
	ListOpenPROptions = struct {
		Owner string
		Repo  string
		Head  string
		Base  string
	}

	// CreatePROptions opens a new pull request.
	CreatePROptions = struct {
		Owner string
		Repo  string
		Title string
		Head  string
		Base  string
		Body  string
	}

	// EditPRBodyOptions replaces an existing pull request body.
	EditPRBodyOptions = struct {
		Owner  string
		Repo   string
		Body   string
		Number int
	}

	createPRBody = struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
	}

	editPRBody = struct {
		Body string `json:"body"`
	}

	jsonCall = struct {
		payload any
		dest    any
		method  string
		path    string
	}

	listPROpts = ListOpenPROptions
)
