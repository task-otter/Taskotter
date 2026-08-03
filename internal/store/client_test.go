// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package store_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/store"
)

const (
	storeRepoPath  = "/repos/task-otter/store"
	storeTagPath   = "/repos/task-otter/store/git/ref/tags/v1.2.3"
	storeCommitSHA = "abc123"

	bodyDefaultBranchMain       = `{"default_branch":"main"}`
	pathCommitsMain             = "/repos/task-otter/store/commits/main"
	bodyShaAbc123Def456         = `{"sha":"abc123def456"}`
	testMainBranchName          = "main"
	testTagV123                 = "v1.2.3"
	bodyEmptyJSON               = "{}"
	caseInvalidJSON             = "invalid json"
	bodyMalformedJSON           = "{"
	errExpectedResolveRef       = "expected ResolveRef error"
	headerAuthorization         = "Authorization"
	errExpectedDownloadSnapshot = "expected DownloadSnapshot error"
	testInternalSkipfiles       = "internal/skipfiles"
	testToken                   = "token"
)

func storeRefInfo(commit string) store.RefInfo {
	return store.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: consts.Empty,
		SourceRef:        consts.Empty,
		ResolvedCommit:   commit,
		DefaultBranch:    consts.Empty,
	}
}

func buildStoreTarGz(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	entries := storeTarEntries()

	for name := range entries {
		writeTarEntry(t, tarWriter, name, entries[name])
	}

	closeTarGzWriters(t, tarWriter, gzipWriter)

	return buf.Bytes()
}

func storeTarEntries() map[string][]byte {
	return map[string][]byte{
		"store-main/taskfiles/go/Taskfile.yml": []byte("version: \"3\"\n"),
		"store-main/.deps.yml":                 []byte("go: []\n"),
	}
}

func writeTarEntry(t *testing.T, tarWriter *tar.Writer, name string, content []byte) {
	t.Helper()

	err := tarWriter.WriteHeader(newTarHeader(name, len(content)))
	if err != nil {
		t.Fatal(err)
	}

	_, err = tarWriter.Write(content)
	if err != nil {
		t.Fatal(err)
	}
}

func newTarHeader(name string, size int) *tar.Header {
	header := new(tar.Header)

	header.Name = name
	header.Mode = consts.FilePerm644
	header.Size = int64(size)
	header.Typeflag = tar.TypeReg

	return header
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

func newStoreTestClient(
	t *testing.T,
	handler http.HandlerFunc,
) (client *store.Client, cleanup func()) {
	t.Helper()

	srv := httptest.NewServer(handler)

	client = store.NewClientWithHTTP(t.Context(), testToken, srv.Client()).
		WithBaseURL(srv.URL)

	return client, srv.Close
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

func writeTestResponse(t *testing.T, writer http.ResponseWriter, body string) {
	t.Helper()

	_, err := writer.Write([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
}

// TestResolveRefDefaultBranch verifies an empty version resolves to the default branch ref and SHA.
func TestResolveRefDefaultBranch(t *testing.T) {
	t.Parallel()

	client, cleanup := newStoreTestClient(
		t,
		defaultBranchHandler(t, bodyDefaultBranchMain, bodyShaAbc123Def456),
	)

	defer cleanup()

	ref, err := client.ResolveRef(t.Context(), consts.Empty)
	if err != nil {
		t.Fatal(err)
	}

	if ref.SourceRef != "refs/heads/main" {
		t.Fatalf("SourceRef = %q", ref.SourceRef)
	}

	if ref.ResolvedCommit != "abc123def456" {
		t.Fatalf("ResolvedCommit = %q", ref.ResolvedCommit)
	}
}

// TestResolveRefDefaultBranchIgnoresNonStringMetadata verifies non-string repo metadata fields are ignored.
func TestResolveRefDefaultBranchIgnoresNonStringMetadata(t *testing.T) {
	t.Parallel()

	client, cleanup := newStoreTestClient(
		t,
		defaultBranchHandler(t, `{"id":12345,"default_branch":"main"}`, bodyShaAbc123Def456),
	)

	defer cleanup()

	ref, err := client.ResolveRef(t.Context(), consts.Empty)
	if err != nil {
		t.Fatal(err)
	}

	if ref.DefaultBranch != testMainBranchName {
		t.Fatalf("DefaultBranch = %q, want main", ref.DefaultBranch)
	}
}

// TestResolveMissingTag verifies resolving a nonexistent tag returns an error.
func TestResolveMissingTag(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case storeRepoPath:
			writeTestResponse(t, writer, bodyDefaultBranchMain)
		case "/repos/task-otter/store/git/ref/tags/v9.9.9":
			http.NotFound(writer, req)
		default:
			http.NotFound(writer, req)
		}
	}))

	defer srv.Close()

	client := store.NewClientWithHTTP(t.Context(), testToken, srv.Client()).
		WithBaseURL(srv.URL)

	_, err := client.ResolveRef(t.Context(), "v9.9.9")
	if err == nil {
		t.Fatal("expected missing tag error")
	}
}

func tagResolutionHandler(
	t *testing.T,
	repoBody string,
	tagResponses map[string]string,
) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Path == storeRepoPath {
			writeTestResponse(t, writer, repoBody)

			return
		}

		if body, ok := tagResponses[req.URL.Path]; ok {
			writeTestResponse(t, writer, body)

			return
		}

		http.NotFound(writer, req)
	}
}

// TestResolveLightweightTag verifies a lightweight tag resolves to its commit SHA.
func TestResolveLightweightTag(t *testing.T) {
	t.Parallel()

	handler := tagResolutionHandler(t, bodyDefaultBranchMain, map[string]string{
		storeTagPath: `{"object":{"sha":"tagsha","type":"commit"}}`,
	})

	client, cleanup := newStoreTestClient(t, handler)

	defer cleanup()

	ref, err := client.ResolveRef(t.Context(), testTagV123)
	if err != nil {
		t.Fatal(err)
	}

	if ref.SourceRef != "refs/tags/v1.2.3" || ref.ResolvedCommit != "tagsha" {
		t.Fatalf("ResolveRef() = %#v", ref)
	}
}

// TestResolveAnnotatedTag verifies an annotated tag resolves to its peeled commit SHA.
func TestResolveAnnotatedTag(t *testing.T) {
	t.Parallel()

	handler := tagResolutionHandler(t, bodyDefaultBranchMain, map[string]string{
		storeTagPath: `{"object":{"sha":"annotated","type":"tag"}}`,
		"/repos/task-otter/store/git/tags/annotated": `{"object":{"sha":"peeled"}}`,
	})

	client, cleanup := newStoreTestClient(t, handler)

	defer cleanup()

	ref, err := client.ResolveRef(t.Context(), testTagV123)
	if err != nil {
		t.Fatal(err)
	}

	if ref.ResolvedCommit != "peeled" {
		t.Fatalf("ResolvedCommit = %q, want peeled", ref.ResolvedCommit)
	}
}

// TestResolveRefErrors verifies repo metadata failures and malformed responses return errors.
func TestResolveRefErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		body   string
		status int
	}{
		{name: "metadata not ok", status: http.StatusInternalServerError, body: bodyEmptyJSON},
		{name: "empty default branch", status: http.StatusOK, body: `{"default_branch":""}`},
		{name: caseInvalidJSON, status: http.StatusOK, body: bodyMalformedJSON},
	}

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assertResolveRefStatusError(t, testCase.status, testCase.body)
		})
	}
}

func assertResolveRefStatusError(t *testing.T, status int, body string) {
	t.Helper()

	client, cleanup := newStoreTestClient(t, statusBodyHandler(t, storeRepoPath, status, body))

	defer cleanup()

	_, err := client.ResolveRef(t.Context(), consts.Empty)
	if err == nil {
		t.Fatal(errExpectedResolveRef)
	}
}

func statusBodyHandler(t *testing.T, path string, status int, body string) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Path != path {
			http.NotFound(writer, req)

			return
		}

		writer.WriteHeader(status)

		writeTestResponse(t, writer, body)
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

func assertResolveTagStatusError(t *testing.T, status int, body string) {
	t.Helper()

	client, cleanup := newStoreTestClient(t, tagStatusHandler(t, status, body))

	defer cleanup()

	_, err := client.ResolveRef(t.Context(), testTagV123)
	if err == nil {
		t.Fatal(errExpectedResolveRef)
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
		if req.URL.Path != "/repos/task-otter/store/tarball/abc123" {
			http.NotFound(writer, req)

			return
		}

		if req.Header.Get(headerAuthorization) != "Bearer token" {
			t.Fatalf("Authorization = %q", req.Header.Get(headerAuthorization))
		}

		_, err := writer.Write(data)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func downloadSnapshotOK(t *testing.T, client *store.Client, ref *store.RefInfo) *store.Snapshot {
	t.Helper()

	snap, err := client.DownloadSnapshot(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func closeAndAssertRemoved(t *testing.T, snap *store.Snapshot) {
	t.Helper()

	err := snap.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(snap.RootDir)

	if !os.IsNotExist(err) {
		t.Fatalf("snapshot root still exists or unexpected stat error: %v", err)
	}
}

// TestNewClientAndDownloadSnapshot verifies a snapshot downloads and its catalog is loaded.
func TestNewClientAndDownloadSnapshot(t *testing.T) {
	t.Parallel()

	if store.NewClient(t.Context(), testToken) == nil {
		t.Fatal("NewClient() returned nil")
	}

	data := buildStoreTarGz(t)

	client, cleanup := newStoreTestClient(t, tarballHandler(t, data))

	defer cleanup()

	ref := storeRefInfo(storeCommitSHA)
	snap := downloadSnapshotOK(t, client, &ref)

	if _, ok := snap.Catalog["go"]; !ok {
		t.Fatalf("Catalog = %#v, want go", snap.Catalog)
	}

	closeAndAssertRemoved(t, snap)
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

func assertDownloadSnapshotStatusError(t *testing.T, status int) {
	t.Helper()

	client, cleanup := newStoreTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
	})

	defer cleanup()

	ref := storeRefInfo(storeCommitSHA)

	_, err := client.DownloadSnapshot(t.Context(), &ref)
	if err == nil {
		t.Fatal(errExpectedDownloadSnapshot)
	}
}

// TestDownloadSnapshotInvalidArchiveCleansUp verifies a non-gzip response errors and leaves no snapshot dir.
func TestDownloadSnapshotInvalidArchiveCleansUp(t *testing.T) {
	t.Parallel()

	client, cleanup := newStoreTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		writeTestResponse(t, writer, "not a gzip archive")
	})

	defer cleanup()

	ref := storeRefInfo(storeCommitSHA)

	_, err := client.DownloadSnapshot(t.Context(), &ref)
	if err == nil {
		t.Fatal(errExpectedDownloadSnapshot)
	}
}

// TestLocalSnapshotLoadsFixture verifies a local fixture directory loads into a snapshot catalog.
func TestLocalSnapshotLoadsFixture(t *testing.T) {
	t.Parallel()

	root := "../../tests/fixtures/store"
	snap := loadLocalFixtureSnapshot(t, root)

	assertFixtureCatalog(t, snap, root)
}

func loadLocalFixtureSnapshot(t *testing.T, root string) *store.Snapshot {
	t.Helper()

	snap, err := store.LocalSnapshot(root, &store.RefInfo{
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

func assertFixtureCatalog(t *testing.T, snap *store.Snapshot, root string) {
	t.Helper()

	if _, ok := snap.Catalog["go"]; !ok {
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
func assertFixtureNamespacedModule(t *testing.T, snap *store.Snapshot, root string) {
	t.Helper()

	if _, ok := snap.Catalog[testInternalSkipfiles]; !ok {
		t.Fatalf("expected namespaced module in catalog: %#v", snap.Catalog)
	}

	if _, ok := snap.Catalog["internal"]; ok {
		t.Fatal("namespace directory must not be cataloged as a module")
	}

	wantDir := filepath.Join(root, "taskfiles", "internal", "skipfiles")

	if snap.ModuleDir(testInternalSkipfiles) != wantDir {
		t.Fatalf("unexpected module dir: %s", snap.ModuleDir(testInternalSkipfiles))
	}
}
