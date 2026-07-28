// Package store downloads and loads TaskOtter store snapshots from GitHub.
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
	"github.com/task-otter/Taskotter/internal/pathutil"
	"gopkg.in/yaml.v3"
)

const (
	storeOwner = "task-otter"
	storeRepo  = "store"

	httpClientTimeout = 60 * time.Second
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

// RefInfo describes a resolved store ref and commit.
type RefInfo struct {
	Repository       string
	RequestedVersion string
	SourceRef        string
	ResolvedCommit   string
	DefaultBranch    string
}

// Snapshot holds an extracted store tree and module metadata.
type Snapshot struct {
	RootDir string
	Catalog map[string]struct{}
	Deps    map[string][]string
	Ref     RefInfo
	cleanup func() error
}

// Close removes temporary snapshot files when present.
func (s *Snapshot) Close() error {
	if s.cleanup != nil {
		return s.cleanup()
	}

	return nil
}

// ModuleDir returns the on-disk path for a source module directory.
func (s *Snapshot) ModuleDir(sourceModule string) string {
	return filepath.Join(s.RootDir, "taskfiles", sourceModule)
}

// HTTPDoer performs HTTP requests for the store client.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client resolves store refs and downloads store archives from GitHub.
type Client struct {
	httpClient HTTPDoer
	token      string
	baseURL    string
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
		baseURL: "https://api.github.com",
	}
}

// NewClientWithHTTP returns a store client that uses a custom HTTP doer.
func NewClientWithHTTP(ctx context.Context, token string, httpClient HTTPDoer) *Client {
	_ = ctx

	return &Client{
		httpClient: httpClient,
		token:      token,
		baseURL:    "https://api.github.com",
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
		return RefInfo{}, err
	}

	info := RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: requestedVersion,
		SourceRef:        "",
		ResolvedCommit:   "",
		DefaultBranch:    defaultBranch,
	}

	if requestedVersion == "" {
		sha, resolveErr := c.resolveBranchHead(ctx, defaultBranch)
		if resolveErr != nil {
			return RefInfo{}, resolveErr
		}

		info.SourceRef = "refs/heads/" + defaultBranch
		info.ResolvedCommit = sha

		return info, nil
	}

	sha, resolveErr := c.resolveTag(ctx, requestedVersion)
	if resolveErr != nil {
		return RefInfo{}, resolveErr
	}

	info.SourceRef = "refs/tags/" + requestedVersion
	info.ResolvedCommit = sha

	return info, nil
}

// DownloadSnapshot downloads and extracts a store archive for the given ref.
func (c *Client) DownloadSnapshot(ctx context.Context, ref RefInfo) (*Snapshot, error) {
	downloadURL := c.apiURL(
		fmt.Sprintf("/repos/%s/%s/tarball/%s", storeOwner, storeRepo, ref.ResolvedCommit),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download store archive: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	err = archiveDownloadStatusError(resp.StatusCode)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "taskotter-store-*")
	if err != nil {
		return nil, fmt.Errorf("create temp directory: %w", err)
	}

	cleanup := func() error { return os.RemoveAll(tmpDir) }

	root, err := archive.ExtractTarGz(resp.Body, tmpDir)
	if err != nil {
		_ = cleanup()

		return nil, fmt.Errorf("extract store archive: %w", err)
	}

	catalog, err := loadCatalog(root)
	if err != nil {
		_ = cleanup()

		return nil, err
	}

	deps, err := loadDeps(root, catalog)
	if err != nil {
		_ = cleanup()

		return nil, err
	}

	return &Snapshot{
		RootDir: root,
		Catalog: catalog,
		Deps:    deps,
		Ref:     ref,
		cleanup: cleanup,
	}, nil
}

func archiveDownloadStatusError(statusCode int) error {
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return fmt.Errorf("%w (HTTP %d)", errArchiveAuthFailed, statusCode)
	case statusCode == http.StatusTooManyRequests:
		return errArchiveRateLimit
	case statusCode != http.StatusOK:
		return fmt.Errorf("%w with HTTP %d", errArchiveDownloadFailed, statusCode)
	default:
		return nil
	}
}

func (c *Client) apiURL(path string) string {
	return c.baseURL + path
}

func loadCatalog(root string) (map[string]struct{}, error) {
	catalog := make(map[string]struct{})

	err := collectModules(filepath.Join(root, "taskfiles"), "", catalog)
	if err != nil {
		return nil, err
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

	if prefix != "" {
		if isModuleDir(entries) {
			catalog[prefix] = struct{}{}
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := path.Join(prefix, entry.Name())
		child := filepath.Join(dir, entry.Name())

		err = collectModules(child, name, catalog)
		if err != nil {
			return err
		}
	}

	return nil
}

func isModuleDir(entries []os.DirEntry) bool {
	for _, entry := range entries {
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
	data, err := pathutil.ReadRelativeFile(root, ".deps.yml")
	if err != nil {
		return nil, fmt.Errorf("read .deps.yml: %w", err)
	}

	var raw map[string][]string

	err = yaml.Unmarshal(data, &raw)
	if err != nil {
		return nil, fmt.Errorf("parse .deps.yml: %w", err)
	}

	for module, deps := range raw {
		if _, ok := catalog[module]; !ok {
			return nil, fmt.Errorf("%w %q", errDepsMissingModule, module)
		}

		for _, dep := range deps {
			if _, ok := catalog[dep]; !ok {
				return nil, fmt.Errorf("%w %q for module %q", errDepsMissingDependency, dep, module)
			}
		}
	}

	return raw, nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "TaskOtter")

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func (c *Client) getJSON(ctx context.Context, path string, payload any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(path), nil)
	if err != nil {
		return fmt.Errorf("create API request: %w", err)
	}

	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s returned HTTP %d", errGitHubAPIFailed, path, resp.StatusCode)
	}

	return decodeJSON(resp.Body, payload)
}

func (c *Client) getDefaultBranch(ctx context.Context) (string, error) {
	payload := make(map[string]string)

	path := fmt.Sprintf("/repos/%s/%s", storeOwner, storeRepo)

	err := c.getJSON(ctx, path, &payload)
	if err != nil {
		return "", fmt.Errorf("fetch store repository metadata: %w", err)
	}

	defaultBranch := payload["default_branch"]
	if defaultBranch == "" {
		return "", errDefaultBranchEmpty
	}

	return defaultBranch, nil
}

func (c *Client) resolveBranchHead(ctx context.Context, branch string) (string, error) {
	var payload struct {
		SHA string `json:"sha"`
	}

	path := fmt.Sprintf("/repos/%s/%s/commits/%s", storeOwner, storeRepo, url.PathEscape(branch))

	err := c.getJSON(ctx, path, &payload)
	if err != nil {
		return "", fmt.Errorf("resolve branch %q: %w", branch, err)
	}

	return payload.SHA, nil
}

func (c *Client) resolveTag(ctx context.Context, tag string) (string, error) {
	var payload struct {
		Object struct {
			SHA  string `json:"sha"`
			Type string `json:"type"`
		} `json:"object"`
	}

	path := fmt.Sprintf("/repos/%s/%s/git/ref/tags/%s", storeOwner, storeRepo, url.PathEscape(tag))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(path), nil)
	if err != nil {
		return "", fmt.Errorf("create tag request: %w", err)
	}

	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve tag request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: %q", errStoreTagNotFound, tag)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w %q with HTTP %d", errResolveTagFailed, tag, resp.StatusCode)
	}

	err = decodeJSON(resp.Body, &payload)
	if err != nil {
		return "", fmt.Errorf("decode tag response: %w", err)
	}

	sha := payload.Object.SHA
	if payload.Object.Type == "tag" {
		sha, err = c.peelAnnotatedTag(ctx, sha)
		if err != nil {
			return "", err
		}
	}

	return sha, nil
}

func (c *Client) peelAnnotatedTag(ctx context.Context, sha string) (string, error) {
	var payload struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}

	path := fmt.Sprintf("/repos/%s/%s/git/tags/%s", storeOwner, storeRepo, sha)

	err := c.getJSON(ctx, path, &payload)
	if err != nil {
		return "", fmt.Errorf("resolve annotated tag: %w", err)
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
func LocalSnapshot(root string, ref RefInfo) (*Snapshot, error) {
	catalog, err := loadCatalog(root)
	if err != nil {
		return nil, err
	}

	deps, err := loadDeps(root, catalog)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		RootDir: root,
		Catalog: catalog,
		Deps:    deps,
		Ref:     ref,
		cleanup: nil,
	}, nil
}
