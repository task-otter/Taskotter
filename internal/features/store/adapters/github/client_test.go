// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/store/adapters/github"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	tarEntry struct {
		writer  *tar.Writer
		name    string
		content []byte
	}

	tagPathRequest struct {
		writer       http.ResponseWriter
		req          *http.Request
		tagResponses map[string]string
	}

	statusHandlerArgs struct {
		path   string
		body   string
		status int
	}

	refErrCase struct {
		name   string
		body   string
		status int
	}
)

const (
	storeRepoPath = "/repos/task-otter/store"

	storeTagPath = "/repos/task-otter/store/git/ref/tags/v1.2.3"

	storeCommitSHA = "abc123"

	bodyDefaultBranchMain = `{"default_branch":"main"}`

	pathCommitsMain = "/repos/task-otter/store/commits/main"

	bodyShaAbc123Def456 = `{"sha":"abc123def456"}`

	testMainBranchName = "main"

	testTagV123 = "v1.2.3"

	bodyEmptyJSON = "{}"

	caseInvalidJSON = "invalid json"

	bodyMalformedJSON = "{"

	errExpectedResolveRef = "expected ResolveRef error"

	headerAuthorization = "Authorization"

	errExpectedDownloadSnapshot = "expected DownloadSnapshot error"

	testInternalSkipfiles = "internal/skipfiles"

	testInternalNamespace = "internal"

	testToken = "token"
)

func resolveRefErrorCases() []refErrCase {
	return []refErrCase{
		{name: "metadata not ok", status: http.StatusInternalServerError, body: bodyEmptyJSON},
		{name: "empty default branch", status: http.StatusOK, body: `{"default_branch":""}`},
		{name: caseInvalidJSON, status: http.StatusOK, body: bodyMalformedJSON},
	}
}

// TestDownloadSnapshotInvalidArchiveCleansUp verifies a non-gzip response errors and leaves no snapshot dir.
func TestDownloadSnapshotInvalidArchiveCleansUp(t *testing.T) {
	t.Parallel()

	client := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		iox.Discard(req)
		writeTestResponse(t, writer, "not a gzip archive")
	})

	ref := storeRefInfo(storeCommitSHA)

	snap, err := client.DownloadSnapshot(t.Context(), &ref)
	iox.Discard(snap)

	if err == nil {
		t.Fatal(errExpectedDownloadSnapshot)
	}
}

// TestDownloadSnapshotStatusErrors verifies non-2xx tarball responses return download errors.
func TestDownloadSnapshotStatusErrors(t *testing.T) {
	t.Parallel()

	statuses := []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	}

	for i := range statuses {
		status := statuses[i]
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			assertDownloadSnapshotStatusError(t, status)
		})
	}
}

// TestLocalSnapshotLoadsFixture verifies a local fixture directory loads into a snapshot catalog.
func TestLocalSnapshotLoadsFixture(t *testing.T) {
	t.Parallel()

	root := "../../../../../tests/fixtures/store"
	snap := loadLocalFixtureSnapshot(t, root)

	assertFixtureCatalog(t, snap, root)
}

// TestNewClientAndDownloadSnapshot verifies a snapshot downloads and its catalog is loaded.
func TestNewClientAndDownloadSnapshot(t *testing.T) {
	t.Parallel()

	if github.NewClient(t.Context(), testToken) == nil {
		t.Fatal("NewClient() returned nil")
	}

	data := buildStoreTarGz(t)

	client := newStoreTestClient(t, tarballHandler(t, data))

	assertDownloadedSnapshot(t, client)
}

// TestResolveAnnotatedTag verifies an annotated tag resolves to its peeled commit SHA.
func TestResolveAnnotatedTag(t *testing.T) {
	t.Parallel()

	handler := tagResolutionHandler(t, bodyDefaultBranchMain, map[string]string{
		storeTagPath: `{"object":{"sha":"annotated","type":"tag"}}`,
		"/repos/task-otter/store/git/tags/annotated": `{"object":{"sha":"peeled"}}`,
	})

	client := newStoreTestClient(t, handler)

	ref, err := client.ResolveRef(t.Context(), testTagV123)
	if err != nil {
		t.Fatal(err)
	}

	if ref.ResolvedCommit != "peeled" {
		t.Fatalf("ResolvedCommit = %q, want peeled", ref.ResolvedCommit)
	}
}

// TestResolveLightweightTag verifies a lightweight tag resolves to its commit SHA.
func TestResolveLightweightTag(t *testing.T) {
	t.Parallel()

	handler := tagResolutionHandler(t, bodyDefaultBranchMain, map[string]string{
		storeTagPath: `{"object":{"sha":"tagsha","type":"commit"}}`,
	})

	client := newStoreTestClient(t, handler)

	ref, err := client.ResolveRef(t.Context(), testTagV123)
	if err != nil {
		t.Fatal(err)
	}

	if ref.SourceRef != "refs/tags/v1.2.3" || ref.ResolvedCommit != "tagsha" {
		t.Fatalf("ResolveRef() = %#v", ref)
	}
}

// TestResolveMissingTag verifies resolving a nonexistent tag returns an error.
func TestResolveMissingTag(t *testing.T) {
	t.Parallel()

	client := newStoreTestClient(t, missingTagHandler(t))

	ref, err := client.ResolveRef(t.Context(), "v9.9.9")
	iox.Discard(ref)

	if err == nil {
		t.Fatal("expected missing tag error")
	}
}

// TestResolveRefDefaultBranch verifies an empty version resolves to the default branch ref and SHA.
func TestResolveRefDefaultBranch(t *testing.T) {
	t.Parallel()

	client := newStoreTestClient(
		t,
		defaultBranchHandler(t, bodyDefaultBranchMain, bodyShaAbc123Def456),
	)

	ref, err := client.ResolveRef(t.Context(), consts.Empty)
	if err != nil {
		t.Fatal(err)
	}

	assertDefaultBranchRef(t, &ref)
}

// TestResolveRefDefaultBranchIgnoresNonStringMetadata verifies non-string repo metadata fields are ignored.
func TestResolveRefDefaultBranchIgnoresNonStringMetadata(t *testing.T) {
	t.Parallel()

	client := newStoreTestClient(
		t,
		defaultBranchHandler(t, `{"id":12345,"default_branch":"main"}`, bodyShaAbc123Def456),
	)

	ref, err := client.ResolveRef(t.Context(), consts.Empty)
	if err != nil {
		t.Fatal(err)
	}

	if ref.DefaultBranch != testMainBranchName {
		t.Fatalf("DefaultBranch = %q, want main", ref.DefaultBranch)
	}
}

// TestResolveRefErrors verifies repo metadata failures and malformed responses return errors.
func TestResolveRefErrors(t *testing.T) {
	t.Parallel()

	cases := resolveRefErrorCases()

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assertResolveRefStatusError(t, &statusHandlerArgs{
				path: storeRepoPath, status: testCase.status, body: testCase.body,
			})
		})
	}
}

// TestResolveTagErrors verifies tag lookup failures and malformed responses return errors.
func TestResolveTagErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		body   string
		status int
	}{
		{name: "server error", status: http.StatusInternalServerError, body: bodyEmptyJSON},
		{name: caseInvalidJSON, status: http.StatusOK, body: bodyMalformedJSON},
	}

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assertResolveTagStatusError(t, testCase.status, testCase.body)
		})
	}
}

func assertDefaultBranchRef(t *testing.T, ref *github.RefInfo) {
	t.Helper()

	if ref.SourceRef != "refs/heads/main" {
		t.Fatalf("SourceRef = %q", ref.SourceRef)
	}

	if ref.ResolvedCommit != "abc123def456" {
		t.Fatalf("ResolvedCommit = %q", ref.ResolvedCommit)
	}
}

func assertDownloadSnapshotStatusError(t *testing.T, status int) {
	t.Helper()

	client := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		iox.Discard(req)
		writer.WriteHeader(status)
	})

	ref := storeRefInfo(storeCommitSHA)

	snap, err := client.DownloadSnapshot(t.Context(), &ref)
	iox.Discard(snap)

	if err == nil {
		t.Fatal(errExpectedDownloadSnapshot)
	}
}

func assertDownloadedSnapshot(t *testing.T, client *github.Client) {
	t.Helper()

	ref := storeRefInfo(storeCommitSHA)
	snap := downloadSnapshotOK(t, client, &ref)

	entry, ok := snap.Catalog[consts.Go]
	iox.Discard(entry)

	if !ok {
		t.Fatalf("Catalog = %#v, want go", snap.Catalog)
	}

	closeAndAssertRemoved(t, snap)
}

func assertFixtureCatalog(t *testing.T, snap *github.Snapshot, root string) {
	t.Helper()

	entry, ok := snap.Catalog[consts.Go]
	iox.Discard(entry)

	if !ok {
		t.Fatal("expected go module in catalog")
	}

	if len(snap.Deps["eslint/node/fnm/pnpm"]) != consts.IndexOne {
		t.Fatalf("unexpected deps: %#v", snap.Deps["eslint/node/fnm/pnpm"])
	}

	assertFixtureNamespacedModule(t, snap, root)
}

// assertFixtureNamespacedModule checks that internal/ (which has no Taskfile.yml
// of its own) is treated as a namespace, so only its namespaced children are
// cataloged as modules, and the namespace itself is not.
func assertFixtureNamespacedModule(t *testing.T, snap *github.Snapshot, root string) {
	t.Helper()

	assertCatalogHasModule(t, snap, testInternalSkipfiles)
	assertCatalogMissingModule(t, snap, testInternalNamespace)
	assertModuleDir(t, snap, root)
}

func assertCatalogHasModule(t *testing.T, snap *github.Snapshot, module string) {
	t.Helper()

	entry, ok := snap.Catalog[module]
	iox.Discard(entry)

	if !ok {
		t.Fatalf("expected namespaced module in catalog: %#v", snap.Catalog)
	}
}

func assertCatalogMissingModule(t *testing.T, snap *github.Snapshot, module string) {
	t.Helper()

	entry, ok := snap.Catalog[module]
	iox.Discard(entry)

	if ok {
		t.Fatal("namespace directory must not be cataloged as a module")
	}
}

func assertModuleDir(t *testing.T, snap *github.Snapshot, root string) {
	t.Helper()

	wantDir := filepath.Join(root, "taskfiles", "internal", "skipfiles")

	if snap.ModuleDir(testInternalSkipfiles) != wantDir {
		t.Fatalf("unexpected module dir: %s", snap.ModuleDir(testInternalSkipfiles))
	}
}

func assertResolveRefStatusError(t *testing.T, args *statusHandlerArgs) {
	t.Helper()

	client := newStoreTestClient(t, statusBodyHandler(t, args))

	ref, err := client.ResolveRef(t.Context(), consts.Empty)
	iox.Discard(ref)

	if err == nil {
		t.Fatal(errExpectedResolveRef)
	}
}

func assertResolveTagStatusError(t *testing.T, status int, body string) {
	t.Helper()

	client := newStoreTestClient(t, tagStatusHandler(t, status, body))

	ref, err := client.ResolveRef(t.Context(), testTagV123)
	iox.Discard(ref)

	if err == nil {
		t.Fatal(errExpectedResolveRef)
	}
}

func assertTarballAuth(t *testing.T, req *http.Request) {
	t.Helper()

	if req.Header.Get(headerAuthorization) != "Bearer token" {
		t.Fatalf("Authorization = %q", req.Header.Get(headerAuthorization))
	}
}

func buildStoreTarGz(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	entries := storeTarEntries()

	for name := range entries {
		writeTarEntry(t, &tarEntry{writer: tarWriter, name: name, content: entries[name]})
	}

	closeTarGzWriters(t, tarWriter, gzipWriter)

	return buf.Bytes()
}

func closeAndAssertRemoved(t *testing.T, snap *github.Snapshot) {
	t.Helper()

	err := snap.Close()
	if err != nil {
		t.Fatal(err)
	}

	stat, err := os.Stat(snap.RootDir)
	iox.Discard(stat)

	if !os.IsNotExist(err) {
		t.Fatalf("snapshot root still exists or unexpected stat error: %v", err)
	}
}

func closeTarGzWriters(t *testing.T, tarWriter *tar.Writer, gzipWriter *gzip.Writer) {
	t.Helper()

	err := tarWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = gzipWriter.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func defaultBranchHandler(t *testing.T, repoBody, commitBody string) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case storeRepoPath:
			writeTestResponse(t, writer, repoBody)
		case pathCommitsMain:
			writeTestResponse(t, writer, commitBody)
		default:
			http.NotFound(writer, req)
		}
	}
}

func downloadSnapshotOK(t *testing.T, client *github.Client, ref *github.RefInfo) *github.Snapshot {
	t.Helper()

	snap, err := client.DownloadSnapshot(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func isTarballRequest(req *http.Request) bool {
	return req.URL.Path == "/repos/task-otter/store/tarball/abc123"
}

func loadLocalFixtureSnapshot(t *testing.T, root string) *github.Snapshot {
	t.Helper()

	snap, err := github.LocalSnapshot(root, &github.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: consts.Empty,
		SourceRef:        consts.Empty,
		ResolvedCommit:   consts.Empty,
		DefaultBranch:    testMainBranchName,
	})
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func missingTagHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Path == storeRepoPath {
			writeTestResponse(t, writer, bodyDefaultBranchMain)

			return
		}

		http.NotFound(writer, req)
	}
}

func newStoreTestClient(t *testing.T, handler http.HandlerFunc) *github.Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := github.NewClientWithHTTP(t.Context(), testToken, srv.Client()).
		WithBaseURL(srv.URL)

	return client
}

func newTarHeader(name string, size int) *tar.Header {
	header := new(tar.Header)

	header.Name = name
	header.Mode = consts.FilePerm644
	header.Size = int64(size)
	header.Typeflag = tar.TypeReg

	return header
}

func respondTagPath(t *testing.T, req *tagPathRequest) {
	t.Helper()

	if body, ok := req.tagResponses[req.req.URL.Path]; ok {
		writeTestResponse(t, req.writer, body)

		return
	}

	http.NotFound(req.writer, req.req)
}

func statusBodyHandler(t *testing.T, args *statusHandlerArgs) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Path != args.path {
			http.NotFound(writer, req)

			return
		}

		writer.WriteHeader(args.status)

		writeTestResponse(t, writer, args.body)
	}
}

func storeRefInfo(commit string) github.RefInfo {
	return github.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: consts.Empty,
		SourceRef:        consts.Empty,
		ResolvedCommit:   commit,
		DefaultBranch:    consts.Empty,
	}
}

func storeTarEntries() map[string][]byte {
	return map[string][]byte{
		"store-main/taskfiles/go/Taskfile.yml": []byte("version: \"3\"\n"),
		"store-main/.deps.yml":                 []byte("go: []\n"),
	}
}

func tagResolutionHandler(t *testing.T, repoBody string, tags map[string]string) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Path == storeRepoPath {
			writeTestResponse(t, writer, repoBody)

			return
		}

		respondTagPath(t, &tagPathRequest{
			writer: writer, req: req, tagResponses: tags,
		})
	}
}

func tagStatusHandler(t *testing.T, status int, body string) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case storeRepoPath:
			writeTestResponse(t, writer, bodyDefaultBranchMain)
		case storeTagPath:
			writer.WriteHeader(status)

			writeTestResponse(t, writer, body)
		default:
			http.NotFound(writer, req)
		}
	}
}

func tarballHandler(t *testing.T, data []byte) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if !isTarballRequest(req) {
			http.NotFound(writer, req)

			return
		}

		assertTarballAuth(t, req)
		writeTarballData(t, writer, data)
	}
}

func writeTarEntry(t *testing.T, entry *tarEntry) {
	t.Helper()

	err := entry.writer.WriteHeader(newTarHeader(entry.name, len(entry.content)))
	if err != nil {
		t.Fatal(err)
	}

	written, err := entry.writer.Write(entry.content)
	iox.Discard(written)

	if err != nil {
		t.Fatal(err)
	}
}

func writeTarballData(t *testing.T, writer http.ResponseWriter, data []byte) {
	t.Helper()

	written, err := writer.Write(data)
	iox.Discard(written)

	if err != nil {
		t.Fatal(err)
	}
}

func writeTestResponse(t *testing.T, writer http.ResponseWriter, body string) {
	t.Helper()

	written, err := writer.Write([]byte(body))
	iox.Discard(written)

	if err != nil {
		t.Fatal(err)
	}
}
