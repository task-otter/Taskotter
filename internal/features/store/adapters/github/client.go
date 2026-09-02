// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package github fetches TaskOtter store module snapshots from GitHub.
//
//nolint:funcorder // HTTP helpers and ref resolution grouped by call flow
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/task-otter/Taskotter/internal/features/store/domain"
	storeservice "github.com/task-otter/Taskotter/internal/features/store/service"
	"github.com/task-otter/Taskotter/internal/shared/archive"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	// RefInfo is store ref metadata from the domain package.
	RefInfo = domain.RefInfo
	// Snapshot is an extracted store tree from the domain package.
	Snapshot = domain.Snapshot

	// HTTPDoer performs HTTP requests for the store client.
	HTTPDoer interface {
		Do(req *http.Request) (*http.Response, error)
	}

	httpRsp = *http.Response

	// Client resolves store refs and downloads store archives from GitHub.
	Client struct {
		httpClient HTTPDoer
		token      string
		baseURL    string
	}

	// tagRefPayload is the GitHub API response shape for a tag ref lookup.
	tagRefPayload struct {
		Object tagRefObject `json:"object"`
	}

	tagRefObject struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	}

	annotatedTagPayload struct {
		Object annotatedTagObject `json:"object"`
	}

	annotatedTagObject struct {
		SHA string `json:"sha"`
	}

	branchHeadPayload struct {
		SHA string `json:"sha"`
	}

	extractedStore struct {
		catalog map[string]struct{}
		deps    map[string][]string
		root    string
	}

	versionRefRequest struct {
		info             *RefInfo
		requestedVersion string
		defaultBranch    string
	}

	resolvedRefRequest struct {
		info      *RefInfo
		resolve   func(context.Context) (string, error)
		sourceRef string
		wrapMsg   string
	}
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

var (
	errArchiveAuthFailed = errors.New("authentication failed downloading store archive")

	errArchiveRateLimit = errors.New("GitHub rate limit exceeded downloading store archive")

	errArchiveDownloadFailed = errors.New("download store archive failed")

	errGitHubAPIFailed = errors.New("GitHub API request failed")

	errDefaultBranchEmpty = errors.New("store repository default branch is empty")

	errStoreTagNotFound = errors.New("store tag does not exist")

	errResolveTagFailed = errors.New("resolve tag failed")
)

// NewClient returns a store client authenticated with the given GitHub token.
func NewClient(ctx context.Context, token string) *Client {
	iox.Discard(ctx)

	return &Client{
		httpClient: &http.Client{
			Timeout:       httpClientTimeout,
			Transport:     nil,
			CheckRedirect: nil,
			Jar:           nil,
		},
		token:   token,
		baseURL: defaultBaseURL,
	}
}

// NewClientWithHTTP returns a store client that uses a custom HTTP doer.
func NewClientWithHTTP(ctx context.Context, token string, httpClient HTTPDoer) *Client {
	iox.Discard(ctx)

	return &Client{
		httpClient: httpClient,
		token:      token,
		baseURL:    defaultBaseURL,
	}
}

// LocalSnapshot creates a snapshot from an on-disk directory (tests/fixtures).
func LocalSnapshot(root string, ref *RefInfo) (*Snapshot, error) {
	snap, err := storeservice.LocalSnapshot(root, ref)
	if err != nil {
		return nil, fmt.Errorf("load local snapshot: %w", err)
	}

	return snap, nil
}

func archiveDownloadStatusError(statusCode int) error {
	if isAuthFailureStatus(statusCode) {
		return fmt.Errorf("%w (HTTP %d)", errArchiveAuthFailed, statusCode)
	}

	switch {
	case statusCode == http.StatusTooManyRequests:
		return errArchiveRateLimit
	case statusCode != http.StatusOK:
		return fmt.Errorf("%w with HTTP %d", errArchiveDownloadFailed, statusCode)
	default:
		return nil
	}
}

func buildSnapshot(body io.Reader, ref *RefInfo) (*Snapshot, error) {
	tmpDir, err := os.MkdirTemp("", "taskotter-store-*")
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}

	extracted, err := extractAndLoad(body, tmpDir)
	if err != nil {
		return nil, fmt.Errorf(fmtExtractAndLoadErr, cleanupTempDirAfterError(tmpDir, err))
	}

	return newSnapshot(&extracted, ref, tmpDir), nil
}

func cleanupTempDirAfterError(tmpDir string, extractErr error) error {
	removeErr := os.RemoveAll(tmpDir)
	if removeErr != nil {
		return errors.Join(
			fmt.Errorf(fmtExtractAndLoadErr, extractErr),
			fmt.Errorf("clean up temp directory %q: %w", tmpDir, removeErr),
		)
	}

	return fmt.Errorf(fmtExtractAndLoadErr, extractErr)
}

func checkTagResponseStatus(statusCode int, tag string) error {
	if statusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %q", errStoreTagNotFound, tag)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("%w %q with HTTP %d", errResolveTagFailed, tag, statusCode)
	}

	return nil
}

func closeOnArchiveStatusError(resp *http.Response) error {
	err := archiveDownloadStatusError(resp.StatusCode)
	if err == nil {
		return nil
	}

	// finalizeArchiveStatusError always reports the status error, joined with any
	// drain or close failure.
	return fmt.Errorf("finalize archive status error: %w", finalizeArchiveStatusError(resp, err))
}

func finalizeArchiveStatusError(resp *http.Response, statusErr error) error {
	drainErr := drainResponseBody(resp)
	if drainErr != nil {
		return errors.Join(
			fmt.Errorf(fmtArchiveDownloadStatusErr, statusErr),
			fmt.Errorf("drain archive response body: %w", drainErr),
		)
	}

	closeErr := resp.Body.Close()
	if closeErr != nil {
		return errors.Join(
			fmt.Errorf(fmtArchiveDownloadStatusErr, statusErr),
			fmt.Errorf(fmtCloseResponseBodyErr, closeErr),
		)
	}

	return fmt.Errorf(fmtArchiveDownloadStatusErr, statusErr)
}

func drainArchiveBody(resp *http.Response) (int64, error) {
	written, err := io.Copy(io.Discard, resp.Body)
	iox.Discard(written)

	closeErr := resp.Body.Close()

	if err != nil {
		return written, fmt.Errorf(fmtDrainResponseBodyErr, err)
	}

	if closeErr != nil {
		return written, fmt.Errorf(fmtCloseResponseBodyErr, closeErr)
	}

	return written, nil
}

func drainResponseBody(resp *http.Response) error {
	written, err := io.Copy(io.Discard, resp.Body)
	iox.Discard(written)

	if err != nil {
		return fmt.Errorf(fmtDrainResponseBodyErr, err)
	}

	return nil
}

func decodeJSON(reader io.Reader, payload any) error {
	err := json.NewDecoder(reader).Decode(payload)
	if err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}

	return nil
}

func decodeTagPayload(resp *http.Response, tag string) (tagRefPayload, error) {
	err := checkTagResponseStatus(resp.StatusCode, tag)
	if err != nil {
		return tagRefPayload{}, fmt.Errorf("check tag response status: %w", err)
	}

	var payload tagRefPayload

	err = decodeJSON(resp.Body, &payload)
	if err != nil {
		return tagRefPayload{}, fmt.Errorf("decode tag response: %w", err)
	}

	return payload, nil
}

func extractAndLoad(body io.Reader, tmpDir string) (extractedStore, error) {
	root, err := archive.ExtractTarGz(body, tmpDir)
	if err != nil {
		return extractedStore{}, fmt.Errorf("extract store archive: %w", err)
	}

	catalog, deps, err := storeservice.LoadCatalogAndDeps(root)
	if err != nil {
		return extractedStore{}, fmt.Errorf("load catalog and deps: %w", err)
	}

	return extractedStore{root: root, catalog: catalog, deps: deps}, nil
}

func extractDefaultBranch(payload map[string]json.RawMessage) (string, error) {
	var defaultBranch string

	err := json.Unmarshal(payload["default_branch"], &defaultBranch)
	if err != nil {
		return consts.Empty, fmt.Errorf("decode default branch: %w", err)
	}

	if defaultBranch == consts.Empty {
		return consts.Empty, errDefaultBranchEmpty
	}

	return defaultBranch, nil
}

func isAuthFailureStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

func newRefInfo(requestedVersion, defaultBranch string) RefInfo {
	return RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: requestedVersion,
		SourceRef:        consts.Empty,
		ResolvedCommit:   consts.Empty,
		DefaultBranch:    defaultBranch,
	}
}

func newSnapshot(extracted *extractedStore, ref *RefInfo, tmpDir string) *Snapshot {
	return &Snapshot{
		RootDir: extracted.root,
		Catalog: extracted.catalog,
		Deps:    extracted.deps,
		Ref:     *ref,
		Cleanup: snapshotCleanup(tmpDir),
	}
}

func readJSONResponse(resp *http.Response, reqPath string, payload any) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s returned HTTP %d", errGitHubAPIFailed, reqPath, resp.StatusCode)
	}

	err := decodeJSON(resp.Body, payload)
	if err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	return nil
}

func snapshotCleanup(tmpDir string) func() error {
	return func() error {
		err := os.RemoveAll(tmpDir)
		if err != nil {
			return fmt.Errorf("remove temp directory %q: %w", tmpDir, err)
		}

		return nil
	}
}

// DownloadSnapshot downloads and extracts a store archive for the given ref.
//
// DownloadSnapshot downloads and extracts a store archive for the given ref.
func (client *Client) DownloadSnapshot(ctx context.Context, ref *RefInfo) (*Snapshot, error) {
	resp, err := client.fetchArchive(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("fetch archive: %w", err)
	}

	defer func() { iox.Discard2(drainArchiveBody(resp)) }()

	snapshot, err := buildSnapshot(resp.Body, ref)
	if err != nil {
		return nil, fmt.Errorf("build snapshot: %w", err)
	}

	return snapshot, nil
}

// ResolveRef resolves a requested store version to a commit SHA.
func (client *Client) ResolveRef(ctx context.Context, requestedVersion string) (RefInfo, error) {
	defaultBranch, err := client.getDefaultBranch(ctx)
	if err != nil {
		return RefInfo{}, fmt.Errorf("get default branch: %w", err)
	}

	info := newRefInfo(requestedVersion, defaultBranch)

	err = client.resolveVersionRef(ctx, &versionRefRequest{
		info:             &info,
		requestedVersion: requestedVersion,
		defaultBranch:    defaultBranch,
	})
	if err != nil {
		return RefInfo{}, fmt.Errorf("resolve version ref: %w", err)
	}

	return info, nil
}

// WithBaseURL overrides the GitHub API base URL, primarily for tests.
func (client *Client) WithBaseURL(baseURL string) *Client {
	client.baseURL = strings.TrimRight(baseURL, "/")

	return client
}

func (client *Client) apiURL(reqPath string) string {
	return client.baseURL + reqPath
}

func (client *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "TaskOtter")

	if client.token != consts.Empty {
		req.Header.Set("Authorization", "Bearer "+client.token)
	}
}

func (client *Client) decodeTag(ctx context.Context, rsp httpRsp, val string) (string, error) {
	sha, err := client.tagSHA(ctx, rsp, val)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve tag from payload: %w", err)
	}

	return sha, nil
}

func (client *Client) doGet(ctx context.Context, reqPath string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.apiURL(reqPath), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}

	client.applyHeaders(req)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request: %w", err)
	}

	return resp, nil
}

func (client *Client) fetchArchive(ctx context.Context, ref *RefInfo) (*http.Response, error) {
	downloadPath := fmt.Sprintf(
		"/repos/%s/%s/tarball/%s",
		storeOwner,
		storeRepo,
		ref.ResolvedCommit,
	)

	resp, err := client.doGet(ctx, downloadPath)
	if err != nil {
		return nil, fmt.Errorf("get archive: %w", err)
	}

	err = closeOnArchiveStatusError(resp)
	if err != nil {
		return nil, fmt.Errorf("check archive download status: %w", err)
	}

	return resp, nil
}

func (client *Client) fetchTagRef(ctx context.Context, tag string) (*http.Response, error) {
	reqPath := fmt.Sprintf(
		"/repos/%s/%s/git/ref/tags/%s",
		storeOwner,
		storeRepo,
		url.PathEscape(tag),
	)

	resp, err := client.doGet(ctx, reqPath)
	if err != nil {
		return nil, fmt.Errorf("get tag ref: %w", err)
	}

	return resp, nil
}

func (client *Client) getDefaultBranch(ctx context.Context) (string, error) {
	payload := make(map[string]json.RawMessage)
	reqPath := fmt.Sprintf("/repos/%s/%s", storeOwner, storeRepo)

	err := client.getJSON(ctx, reqPath, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("fetch store repository metadata: %w", err)
	}

	branch, err := extractDefaultBranch(payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("extract default branch: %w", err)
	}

	return branch, nil
}

func (client *Client) getJSON(ctx context.Context, reqPath string, payload any) error {
	resp, err := client.doGet(ctx, reqPath)
	if err != nil {
		return fmt.Errorf("get %q: %w", reqPath, err)
	}

	defer func() { iox.Discard2(drainArchiveBody(resp)) }()

	err = readJSONResponse(resp, reqPath, payload)
	if err != nil {
		return fmt.Errorf("read JSON response: %w", err)
	}

	return nil
}

func (client *Client) peelAnnotatedTag(ctx context.Context, sha string) (string, error) {
	var payload annotatedTagPayload

	reqPath := fmt.Sprintf("/repos/%s/%s/git/tags/%s", storeOwner, storeRepo, sha)

	err := client.getJSON(ctx, reqPath, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve annotated tag: %w", err)
	}

	return payload.Object.SHA, nil
}

func (client *Client) resolveBranchHead(ctx context.Context, branch string) (string, error) {
	var payload branchHeadPayload

	reqPath := fmt.Sprintf("/repos/%s/%s/commits/%s", storeOwner, storeRepo, url.PathEscape(branch))

	err := client.getJSON(ctx, reqPath, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve branch %q: %w", branch, err)
	}

	return payload.SHA, nil
}

func applyResolvedRef(ctx context.Context, req *resolvedRefRequest) error {
	sha, err := req.resolve(ctx)
	if err != nil {
		return fmt.Errorf(fmtApplyResolvedRefErr, req.wrapMsg, err)
	}

	req.info.SourceRef = req.sourceRef
	req.info.ResolvedCommit = sha

	return nil
}

func (client *Client) resolveSHA(ctx context.Context, payload *tagRefPayload) (string, error) {
	if payload.Object.Type != gitObjectTypeTag {
		return payload.Object.SHA, nil
	}

	sha, err := client.peelAnnotatedTag(ctx, payload.Object.SHA)
	if err != nil {
		return consts.Empty, fmt.Errorf("peel annotated tag: %w", err)
	}

	return sha, nil
}

//nolint:nestif,funlen,maintidx // branch/tag ref paths share applyResolvedRef wiring
func (client *Client) resolveVersionRef(ctx context.Context, req *versionRefRequest) error {
	if req.requestedVersion == consts.Empty {
		err := applyResolvedRef(ctx, &resolvedRefRequest{
			info:      req.info,
			sourceRef: "refs/heads/" + req.defaultBranch,
			wrapMsg:   "resolve branch head",
			resolve: func(callCtx context.Context) (string, error) {
				return client.resolveBranchHead(callCtx, req.defaultBranch)
			},
		})
		if err != nil {
			return fmt.Errorf(fmtApplyResolvedRefErr, "resolve branch ref", err)
		}

		return nil
	}

	err := applyResolvedRef(ctx, &resolvedRefRequest{
		info:      req.info,
		sourceRef: "refs/tags/" + req.requestedVersion,
		wrapMsg:   "resolve tag",
		resolve: func(callCtx context.Context) (string, error) {
			return client.resolveTag(callCtx, req.requestedVersion)
		},
	})
	if err != nil {
		return fmt.Errorf(fmtApplyResolvedRefErr, "resolve tag ref", err)
	}

	return nil
}

func (client *Client) resolveTag(ctx context.Context, tag string) (string, error) {
	resp, err := client.fetchTagRef(ctx, tag)
	if err != nil {
		return consts.Empty, fmt.Errorf("fetch tag ref: %w", err)
	}

	defer func() { iox.Discard2(drainArchiveBody(resp)) }()

	sha, err := client.decodeTag(ctx, resp, tag)
	if err != nil {
		return consts.Empty, fmt.Errorf("decode and resolve tag: %w", err)
	}

	return sha, nil
}

func (client *Client) tagSHA(ctx context.Context, rsp httpRsp, val string) (string, error) {
	payload, err := decodeTagPayload(rsp, val)
	if err != nil {
		return consts.Empty, fmt.Errorf("decode tag payload: %w", err)
	}

	sha, err := client.resolveSHA(ctx, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve sha: %w", err)
	}

	return sha, nil
}
