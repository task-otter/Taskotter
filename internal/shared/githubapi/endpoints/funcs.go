// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package endpoints

import (
	"net/url"
	"strconv"
	"strings"
)

const pathSeparator = "/"

// OpenPRQuery contains head and base filters for open pull requests.
type OpenPRQuery struct {
	Head string
	Base string
}

// PullsPath returns the pull-request collection endpoint.
func PullsPath(owner, repository string) string {
	return pathSeparator + "repos" + pathSeparator + owner + pathSeparator + repository + pathSeparator + "pulls"
}

// PullPath returns one pull-request endpoint.
func PullPath(owner, repository string, number int) string {
	return PullsPath(owner, repository) + "/" + strconv.Itoa(number)
}

// ListOpenPRPath returns the endpoint for open pull requests matching head/base.
func ListOpenPRPath(owner, repository string, filters OpenPRQuery) string {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", filters.Head)
	query.Set("base", filters.Base)

	return PullsPath(owner, repository) + "?" + query.Encode()
}

// RelativeURL creates a URL suitable for resolving against the configured API base.
func RelativeURL(pathPart, rawQuery string) *url.URL {
	return &url.URL{Path: pathPart, RawQuery: rawQuery}
}

// SplitPathQuery separates a relative request path and query.
func SplitPathQuery(requestPath string) (pathPart, queryPart string) {
	pathPart = strings.TrimPrefix(requestPath, pathSeparator)

	cutPath, query, found := strings.Cut(pathPart, "?")

	if !found {
		return pathPart, emptyQuery
	}

	return cutPath, query
}

const emptyQuery = ""
