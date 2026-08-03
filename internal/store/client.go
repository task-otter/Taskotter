// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package store fetches TaskOtter store module snapshots from GitHub.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/task-otter/Taskotter/internal/archive"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

type (

	// RefInfo describes a resolved store ref and commit.
	RefInfo struct {
		Repository       string
		RequestedVersion string
		SourceRef        string
		ResolvedCommit   string
		DefaultBranch    string
	}

	// Snapshot holds an extracted store tree and module metadata.
	Snapshot struct {
		Catalog map[string]struct{}
		Deps    map[string][]string
		cleanup func() error
		Ref     RefInfo
		RootDir string
	}

	// HTTPDoer performs HTTP requests for the store client.
	HTTPDoer interface {
		Do(req *http.Request) (*http.Response, error)
	}

	// Client resolves store refs and downloads store archives from GitHub.
	Client struct {
		httpClient HTTPDoer
		token      string
		baseURL    string
	}

	// tagRefPayload is the GitHub API response shape for a tag ref lookup.
	tagRefPayload struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}
)

const (
	storeOwner = "task-otter"
	storeRepo  = "store"

	httpClientTimeout = 60 * time.Second

	defaultBaseURL = "https://api.github.com"
)

var (
	errArchiveAuthFailed     = errors.New("authentication failed downloading store archive")
	errArchiveRateLimit      = errors.New("GitHub rate limit exceeded downloading store archive")
	errArchiveDownloadFailed = errors.New("download store archive failed")
	errDepsMissingModule     = errors.New(".deps.yml references missing module")
	errDepsMissingDependency = errors.New(".deps.yml references missing dependency")
	errGitHubAPIFailed       = errors.New("GitHub API request failed")
	errDefaultBranchEmpty    = errors.New("store repository default branch is empty")
	errStoreTagNotFound      = errors.New("store tag does not exist")
	errResolveTagFailed      = errors.New("resolve tag failed")
)

// Close removes temporary snapshot files when present.
func (s *Snapshot) Close() error {
	if s.cleanup != nil {
		err := s.cleanup()
		if err != nil {
			return fmt.Errorf("clean up snapshot: %w", err)
		}
	}

	return nil
}

// ModuleDir returns the on-disk path for a source module directory.
func (s *Snapshot) ModuleDir(sourceModule string) string {
	return filepath.Join(s.RootDir, config.DefaultTargetFolder, sourceModule)
}

// NewClient returns a store client authenticated with the given GitHub token.
func NewClient(ctx context.Context, token string) *Client {
	_ = ctx

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
	_ = ctx

	return &Client{
		httpClient: httpClient,
		token:      token,
		baseURL:    defaultBaseURL,
	}
}

// WithBaseURL overrides the GitHub API base URL, primarily for tests.
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = strings.TrimRight(baseURL, "/")

	return c
}

// ResolveRef resolves a requested store version to a commit SHA.
func (c *Client) ResolveRef(ctx context.Context, requestedVersion string) (RefInfo, error) {
	defaultBranch, err := c.getDefaultBranch(ctx)
	if err != nil {
		return RefInfo{}, fmt.Errorf("get default branch: %w", err)
	}

	info := RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: requestedVersion,
		SourceRef:        consts.Empty,
		ResolvedCommit:   consts.Empty,
		DefaultBranch:    defaultBranch,
	}

	if requestedVersion == consts.Empty {
		err = c.resolveBranchRef(ctx, &info, defaultBranch)
		if err != nil {
			return RefInfo{}, fmt.Errorf("resolve branch ref: %w", err)
		}

		return info, nil
	}

	err = c.resolveTagRef(ctx, &info, requestedVersion)
	if err != nil {
		return RefInfo{}, fmt.Errorf("resolve tag ref: %w", err)
	}

	return info, nil
}

func (c *Client) resolveBranchRef(
	ctx context.Context,
	info *RefInfo,
	branch string,
) error {
	sha, err := c.resolveBranchHead(ctx, branch)
	if err != nil {
		return fmt.Errorf("resolve branch head: %w", err)
	}

	info.SourceRef = "refs/heads/" + branch
	info.ResolvedCommit = sha

	return nil
}

func (c *Client) resolveTagRef(ctx context.Context, info *RefInfo, tag string) error {
	sha, err := c.resolveTag(ctx, tag)
	if err != nil {
		return fmt.Errorf("resolve tag: %w", err)
	}

	info.SourceRef = "refs/tags/" + tag
	info.ResolvedCommit = sha

	return nil
}

// DownloadSnapshot downloads and extracts a store archive for the given ref.
func (c *Client) DownloadSnapshot(ctx context.Context, ref *RefInfo) (_ *Snapshot, err error) {
	resp, err := c.fetchArchive(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("fetch archive: %w", err)
	}

	defer func() {
		closeErr := resp.Body.Close()

		if closeErr != nil && err == nil {
			err = fmt.Errorf("close store archive response body: %w", closeErr)
		}
	}()

	snapshot, err := buildSnapshot(resp.Body, ref)
	if err != nil {
		return nil, fmt.Errorf("build snapshot: %w", err)
	}

	return snapshot, nil
}

func (c *Client) fetchArchive(ctx context.Context, ref *RefInfo) (*http.Response, error) {
	downloadPath := fmt.Sprintf(
		"/repos/%s/%s/tarball/%s",
		storeOwner,
		storeRepo,
		ref.ResolvedCommit,
	)

	resp, err := c.doGet(ctx, downloadPath)
	if err != nil {
		return nil, fmt.Errorf("get archive: %w", err)
	}

	err = archiveDownloadStatusError(resp.StatusCode)
	if err != nil {
		statusErr := fmt.Errorf("check archive download status: %w", err)

		closeErr := resp.Body.Close()
		if closeErr != nil {
			return nil, errors.Join(statusErr, fmt.Errorf("close response body: %w", closeErr))
		}

		return nil, statusErr
	}

	return resp, nil
}

func buildSnapshot(body io.Reader, ref *RefInfo) (*Snapshot, error) {
	tmpDir, err := os.MkdirTemp("", "taskotter-store-*")
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}

	root, catalog, deps, err := extractAndLoad(body, tmpDir)
	if err != nil {
		removeErr := os.RemoveAll(tmpDir)
		if removeErr != nil {
			return nil, errors.Join(
				fmt.Errorf("extract and load: %w", err),
				fmt.Errorf("clean up temp directory %q: %w", tmpDir, removeErr),
			)
		}

		return nil, fmt.Errorf("extract and load: %w", err)
	}

	return &Snapshot{
		RootDir: root,
		Catalog: catalog,
		Deps:    deps,
		Ref:     *ref,
		cleanup: func() error {
			err := os.RemoveAll(tmpDir)
			if err != nil {
				return fmt.Errorf("remove temp directory %q: %w", tmpDir, err)
			}

			return nil
		},
	}, nil
}

func extractAndLoad(
	body io.Reader,
	tmpDir string,
) (root string, catalog map[string]struct{}, deps map[string][]string, err error) {
	root, err = archive.ExtractTarGz(body, tmpDir)
	if err != nil {
		return consts.Empty, nil, nil, fmt.Errorf("extract store archive: %w", err)
	}

	catalog, deps, err = loadCatalogAndDeps(root)
	if err != nil {
		return consts.Empty, nil, nil, fmt.Errorf("load catalog and deps: %w", err)
	}

	return root, catalog, deps, nil
}

func loadCatalogAndDeps(
	root string,
) (catalog map[string]struct{}, deps map[string][]string, err error) {
	catalog, err = loadCatalog(root)
	if err != nil {
		return nil, nil, fmt.Errorf("load catalog: %w", err)
	}

	deps, err = loadDeps(root, catalog)
	if err != nil {
		return nil, nil, fmt.Errorf("load deps: %w", err)
	}

	return catalog, deps, nil
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

func isAuthFailureStatus(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

func (c *Client) apiURL(reqPath string) string {
	return c.baseURL + reqPath
}

func loadCatalog(root string) (map[string]struct{}, error) {
	catalog := make(map[string]struct{})

	err := collectModules(filepath.Join(root, "taskfiles"), consts.Empty, catalog)
	if err != nil {
		return nil, fmt.Errorf("collect modules: %w", err)
	}

	return catalog, nil
}

// collectModules walks the store taskfiles tree and records module names. A
// directory with a Taskfile.yml is a module; directories without one are
// namespaces whose children may be modules.
func collectModules(dir, prefix string, catalog map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("load module catalog: %w", err)
	}

	if prefix != consts.Empty && isModuleDir(entries) {
		catalog[prefix] = struct{}{}
	}

	err = visitChildDirs(dir, prefix, entries, catalog)
	if err != nil {
		return fmt.Errorf("visit child directories: %w", err)
	}

	return nil
}

func visitChildDirs(dir, prefix string, entries []os.DirEntry, catalog map[string]struct{}) error {
	for i := range entries {
		entry := entries[i]

		if !entry.IsDir() {
			continue
		}

		name := path.Join(prefix, entry.Name())
		child := filepath.Join(dir, entry.Name())

		err := collectModules(child, name, catalog)
		if err != nil {
			return fmt.Errorf("collect modules under %q: %w", name, err)
		}
	}

	return nil
}

func isModuleDir(entries []os.DirEntry) bool {
	for i := range entries {
		entry := entries[i]

		if entry.IsDir() {
			continue
		}

		if entry.Name() == "Taskfile.yml" {
			return true
		}
	}

	return false
}

func loadDeps(root string, catalog map[string]struct{}) (map[string][]string, error) {
	raw, err := parseDepsFile(root)
	if err != nil {
		return nil, fmt.Errorf("parse deps file: %w", err)
	}

	err = validateDeps(raw, catalog)
	if err != nil {
		return nil, fmt.Errorf("validate deps: %w", err)
	}

	return raw, nil
}

func parseDepsFile(root string) (map[string][]string, error) {
	data, err := pathutil.ReadRelativeFile(root, ".deps.yml")
	if err != nil {
		return nil, fmt.Errorf("read .deps.yml: %w", err)
	}

	var raw map[string][]string

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("parse .deps.yml: %w", err)
	}

	return raw, nil
}

func validateDeps(raw map[string][]string, catalog map[string]struct{}) error {
	for module := range raw {
		err := validateModuleDeps(module, raw[module], catalog)
		if err != nil {
			return fmt.Errorf("validate module deps for %q: %w", module, err)
		}
	}

	return nil
}

func validateModuleDeps(module string, deps []string, catalog map[string]struct{}) error {
	if _, ok := catalog[module]; !ok {
		return fmt.Errorf("%w %q", errDepsMissingModule, module)
	}

	for i := range deps {
		dep := deps[i]

		if _, ok := catalog[dep]; !ok {
			return fmt.Errorf("%w %q for module %q", errDepsMissingDependency, dep, module)
		}
	}

	return nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "TaskOtter")

	if c.token != consts.Empty {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *Client) doGet(ctx context.Context, reqPath string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(reqPath), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}

	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request: %w", err)
	}

	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, reqPath string, payload any) (err error) {
	resp, err := c.doGet(ctx, reqPath)
	if err != nil {
		return fmt.Errorf("get %q: %w", reqPath, err)
	}

	defer func() {
		closeErr := resp.Body.Close()

		if closeErr != nil && err == nil {
			err = fmt.Errorf("close response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s returned HTTP %d", errGitHubAPIFailed, reqPath, resp.StatusCode)
	}

	err = decodeJSON(resp.Body, payload)
	if err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	return nil
}

func (c *Client) getDefaultBranch(ctx context.Context) (string, error) {
	payload := make(map[string]json.RawMessage)
	reqPath := fmt.Sprintf("/repos/%s/%s", storeOwner, storeRepo)

	err := c.getJSON(ctx, reqPath, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("fetch store repository metadata: %w", err)
	}

	branch, err := extractDefaultBranch(payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("extract default branch: %w", err)
	}

	return branch, nil
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

func (c *Client) resolveBranchHead(ctx context.Context, branch string) (string, error) {
	var payload struct {
		SHA string `json:"sha"`
	}

	reqPath := fmt.Sprintf("/repos/%s/%s/commits/%s", storeOwner, storeRepo, url.PathEscape(branch))

	err := c.getJSON(ctx, reqPath, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve branch %q: %w", branch, err)
	}

	return payload.SHA, nil
}

func (c *Client) resolveTag(ctx context.Context, tag string) (_ string, err error) {
	reqPath := fmt.Sprintf(
		"/repos/%s/%s/git/ref/tags/%s",
		storeOwner,
		storeRepo,
		url.PathEscape(tag),
	)

	resp, err := c.doGet(ctx, reqPath)
	if err != nil {
		return consts.Empty, fmt.Errorf("get tag ref: %w", err)
	}

	defer func() {
		closeErr := resp.Body.Close()

		if closeErr != nil && err == nil {
			err = fmt.Errorf("close response body: %w", closeErr)
		}
	}()

	payload, err := decodeTagPayload(resp, tag)
	if err != nil {
		return consts.Empty, fmt.Errorf("decode tag payload: %w", err)
	}

	sha, err := c.resolveSHA(ctx, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve sha: %w", err)
	}

	return sha, nil
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

func (c *Client) resolveSHA(ctx context.Context, payload *tagRefPayload) (string, error) {
	if payload.Object.Type != "tag" {
		return payload.Object.SHA, nil
	}

	sha, err := c.peelAnnotatedTag(ctx, payload.Object.SHA)
	if err != nil {
		return consts.Empty, fmt.Errorf("peel annotated tag: %w", err)
	}

	return sha, nil
}

func (c *Client) peelAnnotatedTag(ctx context.Context, sha string) (string, error) {
	var payload struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}

	reqPath := fmt.Sprintf("/repos/%s/%s/git/tags/%s", storeOwner, storeRepo, sha)

	err := c.getJSON(ctx, reqPath, &payload)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve annotated tag: %w", err)
	}

	return payload.Object.SHA, nil
}

func decodeJSON(reader io.Reader, payload any) error {
	err := json.NewDecoder(reader).Decode(payload)
	if err != nil {
		return fmt.Errorf("decode JSON response: %w", err)
	}

	return nil
}

// LocalSnapshot creates a snapshot from an on-disk directory (tests/fixtures).
func LocalSnapshot(root string, ref *RefInfo) (*Snapshot, error) {
	catalog, err := loadCatalog(root)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}

	deps, err := loadDeps(root, catalog)
	if err != nil {
		return nil, fmt.Errorf("load deps: %w", err)
	}

	return &Snapshot{
		RootDir: root,
		Catalog: catalog,
		Deps:    deps,
		Ref:     *ref,
		cleanup: nil,
	}, nil
}
