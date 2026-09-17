// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package github fetches TaskOtter store module snapshots from GitHub.
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

	httpClient := &http.Client{
		Timeout:       httpClientTimeout,
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
	}

	return &Client{fns: clientFns{
		do:      httpClient.Do,
		token:   token,
		baseURL: defaultBaseURL,
	}}
}

// NewClientWithHTTP returns a store client that uses a custom HTTP doer.
func NewClientWithHTTP(ctx context.Context, token string, httpClient HTTPDoer) *Client {
	iox.Discard(ctx)

	return &Client{fns: clientFns{
		do:      httpClient.Do,
		token:   token,
		baseURL: defaultBaseURL,
	}}
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
	iox.Discard(client.fns)

	return downloadSnapshot(ctx, client, ref)
}

func downloadSnapshot(ctx context.Context, client *Client, ref *RefInfo) (*Snapshot, error) {
	resp, err := fetchArchive(ctx, client, ref)
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
	iox.Discard(client.fns)

	return resolveRef(ctx, client, requestedVersion)
}

func resolveRef(ctx context.Context, client *Client, requestedVersion string) (RefInfo, error) {
	defaultBranch, err := getDefaultBranch(ctx, client)
	if err != nil {
		return RefInfo{}, fmt.Errorf("get default branch: %w", err)
	}

	info := newRefInfo(requestedVersion, defaultBranch)

	err = resolveVersionRef(ctx, client, &versionRefRequest{
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
	iox.Discard(client.fns)

	return withBaseURL(client, baseURL)
}

func withBaseURL(client *Client, baseURL string) *Client {
	client.fns.baseURL = strings.TrimRight(baseURL, "/")

	return client
}

func apiURL(client *Client, reqPath string) string {
	return client.fns.baseURL + reqPath
}

func applyHeaders(client *Client, req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "TaskOtter")

	if client.fns.token != consts.Empty {
		req.Header.Set("Authorization", "Bearer "+client.fns.token)
	}
}

func decodeTag(ctx context.Context, client *Client, args decodeTagArgs) (string, error) {
	sha, err := tagSHA(ctx, client, decodeTagArgs{rsp: args.rsp, val: args.val})
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve tag from payload: %w", err)
	}

	return sha, nil
}

func doGet(ctx context.Context, client *Client, reqPath string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		apiURL(client, reqPath),
		http.NoBody,
	)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}

	applyHeaders(client, req)

	resp, err := client.fns.do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request: %w", err)
	}

	return resp, nil
}

func fetchArchive(ctx context.Context, client *Client, ref *RefInfo) (*http.Response, error) {
	downloadPath := fmt.Sprintf(
		"/repos/%s/%s/tarball/%s",
		storeOwner,
		storeRepo,
		ref.ResolvedCommit,
	)

	resp, err := doGet(ctx, client, downloadPath)
	if err != nil {
		return nil, fmt.Errorf("get archive: %w", err)
	}

	err = closeOnArchiveStatusError(resp)
	if err != nil {
		return nil, fmt.Errorf("check archive download status: %w", err)
	}

	return resp, nil
}

func fetchTagRef(ctx context.Context, client *Client, tag string) (*http.Response, error) {
	reqPath := fmt.Sprintf(
		"/repos/%s/%s/git/ref/tags/%s",
		storeOwner,
		storeRepo,
		url.PathEscape(tag),
	)

	resp, err := doGet(ctx, client, reqPath)
	if err != nil {
		return nil, fmt.Errorf("get tag ref: %w", err)
	}

	return resp, nil
}

func getDefaultBranch(ctx context.Context, client *Client) (string, error) {
	payload := make(map[string]json.RawMessage)
	reqPath := fmt.Sprintf("/repos/%s/%s", storeOwner, storeRepo)

	err := getJSON(ctx, client, &getJSONArgs{reqPath: reqPath, payload: &payload})
	if err != nil {
		return consts.Empty, fmt.Errorf("fetch store repository metadata: %w", err)
	}

	branch, err := extractDefaultBranch(payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("extract default branch: %w", err)
	}

	return branch, nil
}

func getJSON(ctx context.Context, client *Client, args *getJSONArgs) error {
	resp, err := doGet(ctx, client, args.reqPath)
	if err != nil {
		return fmt.Errorf("get %q: %w", args.reqPath, err)
	}

	defer func() { iox.Discard2(drainArchiveBody(resp)) }()

	err = readJSONResponse(resp, args.reqPath, args.payload)
	if err != nil {
		return fmt.Errorf("read JSON response: %w", err)
	}

	return nil
}

func peelAnnotatedTag(ctx context.Context, client *Client, sha string) (string, error) {
	var payload annotatedTagPayload

	reqPath := fmt.Sprintf("/repos/%s/%s/git/tags/%s", storeOwner, storeRepo, sha)

	err := getJSON(ctx, client, &getJSONArgs{reqPath: reqPath, payload: &payload})
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve annotated tag: %w", err)
	}

	return payload.Object.SHA, nil
}

func resolveBranchHead(ctx context.Context, client *Client, branch string) (string, error) {
	var payload branchHeadPayload

	reqPath := fmt.Sprintf("/repos/%s/%s/commits/%s", storeOwner, storeRepo, url.PathEscape(branch))

	err := getJSON(ctx, client, &getJSONArgs{reqPath: reqPath, payload: &payload})
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

func resolveSHA(ctx context.Context, client *Client, payload *tagRefPayload) (string, error) {
	if payload.Object.Type != gitObjectTypeTag {
		return payload.Object.SHA, nil
	}

	sha, err := peelAnnotatedTag(ctx, client, payload.Object.SHA)
	if err != nil {
		return consts.Empty, fmt.Errorf("peel annotated tag: %w", err)
	}

	return sha, nil
}

func resolveVersionRef(ctx context.Context, client *Client, req *versionRefRequest) error {
	if req.requestedVersion == consts.Empty {
		err := applyResolvedRef(ctx, &resolvedRefRequest{
			info:      req.info,
			sourceRef: "refs/heads/" + req.defaultBranch,
			wrapMsg:   "resolve branch head",
			resolve: func(callCtx context.Context) (string, error) {
				return resolveBranchHead(callCtx, client, req.defaultBranch)
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
			return resolveTag(callCtx, client, req.requestedVersion)
		},
	})
	if err != nil {
		return fmt.Errorf(fmtApplyResolvedRefErr, "resolve tag ref", err)
	}

	return nil
}

func resolveTag(ctx context.Context, client *Client, tag string) (string, error) {
	resp, err := fetchTagRef(ctx, client, tag)
	if err != nil {
		return consts.Empty, fmt.Errorf("fetch tag ref: %w", err)
	}

	defer func() { iox.Discard2(drainArchiveBody(resp)) }()

	sha, err := decodeTag(ctx, client, decodeTagArgs{rsp: resp, val: tag})
	if err != nil {
		return consts.Empty, fmt.Errorf("decode and resolve tag: %w", err)
	}

	return sha, nil
}

func tagSHA(ctx context.Context, client *Client, args decodeTagArgs) (string, error) {
	payload, err := decodeTagPayload(args.rsp, args.val)
	if err != nil {
		return consts.Empty, fmt.Errorf("decode tag payload: %w", err)
	}

	sha, err := resolveSHA(ctx, client, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve sha: %w", err)
	}

	return sha, nil
}
