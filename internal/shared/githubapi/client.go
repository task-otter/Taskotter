// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package githubapi wraps the GitHub REST API client.
package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"golang.org/x/oauth2"
)

type (

	// Client wraps GitHub pull request API calls.
	Client struct {
		httpClient *http.Client
		baseURL    *url.URL
	}

	// PullRequest is a minimal pull request view.
	PullRequest struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}

	// ListOpenPROptions selects open pull requests for a head/base pair.
	ListOpenPROptions struct {
		Owner string
		Repo  string
		Head  string
		Base  string
	}

	// CreatePROptions opens a new pull request.
	CreatePROptions struct {
		Owner string
		Repo  string
		Title string
		Head  string
		Base  string
		Body  string
	}

	// EditPRBodyOptions replaces an existing pull request body.
	EditPRBodyOptions struct {
		Owner  string
		Repo   string
		Body   string
		Number int
	}

	createPRBody struct {
		Title string `json:"title"`
		Head  string `json:"head"`
		Base  string `json:"base"`
		Body  string `json:"body"`
	}

	editPRBody struct {
		Body string `json:"body"`
	}

	jsonCall struct {
		payload any
		dest    any
		method  string
		path    string
	}

	listPROpts = ListOpenPROptions
)

const (
	defaultAPIHost = "api.github.com"

	fmtListPullRequestsErr = "list pull requests: %w"

	acceptJSON = "application/vnd.github+json"

	apiVersionHeader = "X-Github-Api-Version"

	apiVersion = "2022-11-28"
)

var errGitHubAPIStatus = errors.New("GitHub API status error")

// NewClient builds an authenticated GitHub API client.
func NewClient(ctx context.Context, token string) *Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken:  token,
		TokenType:    "Bearer",
		RefreshToken: "",
		Expiry:       time.Time{},
		ExpiresIn:    0,
	})

	return &Client{
		httpClient: oauth2.NewClient(ctx, tokenSource),
		baseURL:    defaultAPIBaseURL(),
	}
}

// NewClientWithHTTP builds a client that sends requests through httpClient to baseURL.
func NewClientWithHTTP(baseURL string, httpClient *http.Client) (*Client, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse GitHub base URL: %w", err)
	}

	return &Client{httpClient: httpClient, baseURL: parsedURL}, nil
}

func defaultAPIBaseURL() *url.URL {
	return &url.URL{
		Scheme:      "https",
		Opaque:      consts.Empty,
		User:        nil,
		Host:        defaultAPIHost,
		Path:        consts.PathSepString,
		RawPath:     consts.Empty,
		OmitHost:    false,
		ForceQuery:  false,
		RawQuery:    consts.Empty,
		Fragment:    consts.Empty,
		RawFragment: consts.Empty,
	}
}

// CreatePR opens a new pull request.
func (client *Client) CreatePR(ctx context.Context, opts *CreatePROptions) (PullRequest, error) {
	var pull PullRequest

	err := client.doJSON(ctx, &jsonCall{
		method:  http.MethodPost,
		path:    pullsPath(opts.Owner, opts.Repo),
		payload: newCreatePRBody(opts),
		dest:    &pull,
	})
	if err != nil {
		return PullRequest{}, fmt.Errorf("create pull request: %w", err)
	}

	return pull, nil
}

// EditPRBody replaces the body of an existing pull request.
func (client *Client) EditPRBody(ctx context.Context, opts *EditPRBodyOptions) error {
	err := client.doJSON(ctx, &jsonCall{
		method:  http.MethodPatch,
		path:    pullPath(opts.Owner, opts.Repo, opts.Number),
		payload: editPRBody{Body: opts.Body},
		dest:    nil,
	})
	if err != nil {
		return fmt.Errorf("edit pull request: %w", err)
	}

	return nil
}

// ListOpenPRs returns open pull requests matching head and base.
func (client *Client) ListOpenPRs(ctx context.Context, opt *listPROpts) ([]PullRequest, error) {
	var prs []PullRequest

	err := client.doJSON(ctx, &jsonCall{
		method:  http.MethodGet,
		path:    listOpenPRsPath(opt),
		payload: nil,
		dest:    &prs,
	})
	if err != nil {
		return nil, fmt.Errorf(fmtListPullRequestsErr, err)
	}

	out := make([]PullRequest, consts.IndexZero, len(prs))

	return append(out, prs...), nil
}

func (client *Client) apiURL(reqPath string) string {
	pathPart, rawQuery := splitPathQuery(reqPath)

	return client.baseURL.ResolveReference(newRelativeURL(pathPart, rawQuery)).String()
}

func (client *Client) doJSON(ctx context.Context, call *jsonCall) (err error) {
	resp, err := client.doRequest(ctx, call)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}

	defer func() {
		err = appendBodyClose(err, resp)
	}()

	readErr := readJSONBody(resp, call.dest)
	if readErr != nil {
		return fmt.Errorf("read json body: %w", readErr)
	}

	return nil
}

func appendBodyClose(err error, resp *http.Response) error {
	copied, copyErr := io.Copy(io.Discard, resp.Body)
	iox.Discard(copied)

	closeErr := resp.Body.Close()

	if err != nil {
		return err
	}

	if copyErr != nil {
		return fmt.Errorf("drain response body: %w", copyErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close response body: %w", closeErr)
	}

	return nil
}

func (client *Client) doRequest(ctx context.Context, call *jsonCall) (*http.Response, error) {
	req, err := newAPIRequest(ctx, call, client.apiURL(call.path))
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request: %w", err)
	}

	return resp, nil
}

func newCreatePRBody(opts *CreatePROptions) createPRBody {
	return createPRBody{
		Title: opts.Title,
		Head:  opts.Head,
		Base:  opts.Base,
		Body:  opts.Body,
	}
}

func listOpenPRsPath(opts *ListOpenPROptions) string {
	return pullsPath(opts.Owner, opts.Repo) + "?" + listOpenPRQuery(opts).Encode()
}

func listOpenPRQuery(opts *ListOpenPROptions) url.Values {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", opts.Head)
	query.Set("base", opts.Base)

	return query
}

func pullsPath(owner, repo string) string {
	return "/repos/" + owner + "/" + repo + "/pulls"
}

func pullPath(owner, repo string, number int) string {
	return pullsPath(owner, repo) + "/" + strconv.Itoa(number)
}

func newRelativeURL(pathPart, rawQuery string) *url.URL {
	return &url.URL{
		Scheme:      consts.Empty,
		Opaque:      consts.Empty,
		User:        nil,
		Host:        consts.Empty,
		Path:        pathPart,
		RawPath:     consts.Empty,
		OmitHost:    false,
		ForceQuery:  false,
		RawQuery:    rawQuery,
		Fragment:    consts.Empty,
		RawFragment: consts.Empty,
	}
}

func newAPIRequest(ctx context.Context, call *jsonCall, fullURL string) (*http.Request, error) {
	body, err := marshalPayload(call.payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, call.method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	applyHeaders(req, call.payload)

	return req, nil
}

func marshalPayload(payload any) ([]byte, error) {
	if payload == nil {
		return nil, nil
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	return data, nil
}

func readJSONBody(resp *http.Response, dest any) error {
	err := checkResponseStatus(resp.StatusCode)
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}

	err = decodeResponseBody(resp.Body, dest)
	if err != nil {
		return fmt.Errorf("decode body: %w", err)
	}

	return nil
}

func checkResponseStatus(statusCode int) error {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %d", errGitHubAPIStatus, statusCode)
	}

	return nil
}

func decodeResponseBody(body io.Reader, dest any) error {
	if dest == nil {
		return nil
	}

	decodeErr := json.NewDecoder(body).Decode(dest)
	if decodeErr != nil {
		return fmt.Errorf("decode response: %w", decodeErr)
	}

	return nil
}

func applyHeaders(req *http.Request, payload any) {
	req.Header.Set("Accept", acceptJSON)
	req.Header.Set(apiVersionHeader, apiVersion)

	if payload == nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
}

func splitPathQuery(reqPath string) (pathPart, rawQuery string) {
	pathPart = strings.TrimPrefix(reqPath, consts.PathSepString)

	cutPath, cutQuery, found := strings.Cut(pathPart, "?")

	if !found {
		return pathPart, consts.Empty
	}

	return cutPath, cutQuery
}
