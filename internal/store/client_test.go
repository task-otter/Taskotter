package store_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/store"
)

const (
	storeRepoPath  = "/repos/task-otter/store"
	storeTagPath   = "/repos/task-otter/store/git/ref/tags/v1.2.3"
	storeCommitSHA = "abc123"
)

func storeRefInfo(commit string) store.RefInfo {
	return store.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: "",
		SourceRef:        "",
		ResolvedCommit:   commit,
		DefaultBranch:    "",
	}
}

func buildStoreTarGz(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	entries := map[string][]byte{
		"store-main/taskfiles/go/Taskfile.yml": []byte("version: \"3\"\n"),
		"store-main/.deps.yml":                 []byte("go: []\n"),
	}

	for name, content := range entries {
		header := new(tar.Header)
		header.Name = name
		header.Mode = 0o644
		header.Size = int64(len(content))
		header.Typeflag = tar.TypeReg
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}

		err := tarWriter.WriteHeader(header)
		if err != nil {
			t.Fatal(err)
		}

		_, err = tarWriter.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := tarWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = gzipWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func newStoreTestClient(t *testing.T, handler http.HandlerFunc) (*store.Client, func()) {
	t.Helper()

	srv := httptest.NewServer(handler)
	client := store.NewClientWithHTTP(context.Background(), "token", srv.Client()).
		WithBaseURL(srv.URL)

	return client, srv.Close
}

func TestResolveRefDefaultBranch(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case storeRepoPath:
			_, _ = writer.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/task-otter/store/commits/main":
			_, _ = writer.Write([]byte(`{"sha":"abc123def456"}`))
		default:
			http.NotFound(writer, req)
		}
	}))
	defer srv.Close()

	client := store.NewClientWithHTTP(context.Background(), "token", srv.Client()).
		WithBaseURL(srv.URL)

	ref, err := client.ResolveRef(context.Background(), "")
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

func TestResolveMissingTag(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case storeRepoPath:
			_, _ = writer.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/task-otter/store/git/ref/tags/v9.9.9":
			http.NotFound(writer, req)
		default:
			http.NotFound(writer, req)
		}
	}))
	defer srv.Close()

	client := store.NewClientWithHTTP(context.Background(), "token", srv.Client()).
		WithBaseURL(srv.URL)

	_, err := client.ResolveRef(context.Background(), "v9.9.9")
	if err == nil {
		t.Fatal("expected missing tag error")
	}
}

func TestResolveLightweightTag(t *testing.T) {
	t.Parallel()

	client, cleanup := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case storeRepoPath:
			_, _ = writer.Write([]byte(`{"default_branch":"main"}`))
		case storeTagPath:
			_, _ = writer.Write([]byte(`{"object":{"sha":"tagsha","type":"commit"}}`))
		default:
			http.NotFound(writer, req)
		}
	})
	defer cleanup()

	ref, err := client.ResolveRef(context.Background(), "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}

	if ref.SourceRef != "refs/tags/v1.2.3" || ref.ResolvedCommit != "tagsha" {
		t.Fatalf("ResolveRef() = %#v", ref)
	}
}

func TestResolveAnnotatedTag(t *testing.T) {
	t.Parallel()

	client, cleanup := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case storeRepoPath:
			_, _ = writer.Write([]byte(`{"default_branch":"main"}`))
		case storeTagPath:
			_, _ = writer.Write([]byte(`{"object":{"sha":"annotated","type":"tag"}}`))
		case "/repos/task-otter/store/git/tags/annotated":
			_, _ = writer.Write([]byte(`{"object":{"sha":"peeled"}}`))
		default:
			http.NotFound(writer, req)
		}
	})
	defer cleanup()

	ref, err := client.ResolveRef(context.Background(), "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}

	if ref.ResolvedCommit != "peeled" {
		t.Fatalf("ResolvedCommit = %q, want peeled", ref.ResolvedCommit)
	}
}

func TestResolveRefErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "metadata not ok", status: http.StatusInternalServerError, body: `{}`},
		{name: "empty default branch", status: http.StatusOK, body: `{"default_branch":""}`},
		{name: "invalid json", status: http.StatusOK, body: `{`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			client, cleanup := newStoreTestClient(
				t,
				func(writer http.ResponseWriter, req *http.Request) {
					if req.URL.Path != storeRepoPath {
						http.NotFound(writer, req)

						return
					}

					writer.WriteHeader(testCase.status)
					_, _ = writer.Write([]byte(testCase.body))
				},
			)
			defer cleanup()

			_, err := client.ResolveRef(context.Background(), "")
			if err == nil {
				t.Fatal("expected ResolveRef error")
			}
		})
	}
}

func TestResolveTagErrors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "server error", status: http.StatusInternalServerError, body: `{}`},
		{name: "invalid json", status: http.StatusOK, body: `{`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			client, cleanup := newStoreTestClient(
				t,
				func(writer http.ResponseWriter, req *http.Request) {
					switch req.URL.Path {
					case storeRepoPath:
						_, _ = writer.Write([]byte(`{"default_branch":"main"}`))
					case storeTagPath:
						writer.WriteHeader(testCase.status)
						_, _ = writer.Write([]byte(testCase.body))
					default:
						http.NotFound(writer, req)
					}
				},
			)
			defer cleanup()

			_, err := client.ResolveRef(context.Background(), "v1.2.3")
			if err == nil {
				t.Fatal("expected ResolveRef error")
			}
		})
	}
}

func TestNewClientAndDownloadSnapshot(t *testing.T) {
	t.Parallel()

	if store.NewClient(context.Background(), "token") == nil {
		t.Fatal("NewClient() returned nil")
	}

	data := buildStoreTarGz(t)

	client, cleanup := newStoreTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/repos/task-otter/store/tarball/abc123" {
			http.NotFound(writer, req)

			return
		}

		if req.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
		}

		_, _ = writer.Write(data)
	})
	defer cleanup()

	snap, err := client.DownloadSnapshot(
		context.Background(),
		storeRefInfo(storeCommitSHA),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := snap.Catalog["go"]; !ok {
		t.Fatalf("Catalog = %#v, want go", snap.Catalog)
	}

	err = snap.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(snap.RootDir)
	if !os.IsNotExist(err) {
		t.Fatalf("snapshot root still exists or unexpected stat error: %v", err)
	}
}

func TestDownloadSnapshotStatusErrors(t *testing.T) {
	t.Parallel()

	for _, status := range []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			client, cleanup := newStoreTestClient(
				t,
				func(writer http.ResponseWriter, _ *http.Request) {
					writer.WriteHeader(status)
				},
			)
			defer cleanup()

			_, err := client.DownloadSnapshot(
				context.Background(),
				storeRefInfo(storeCommitSHA),
			)
			if err == nil {
				t.Fatal("expected DownloadSnapshot error")
			}
		})
	}
}

func TestDownloadSnapshotInvalidArchiveCleansUp(t *testing.T) {
	t.Parallel()

	client, cleanup := newStoreTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("not a gzip archive"))
	})
	defer cleanup()

	_, err := client.DownloadSnapshot(context.Background(), storeRefInfo(storeCommitSHA))
	if err == nil {
		t.Fatal("expected DownloadSnapshot error")
	}
}

func TestLocalSnapshotLoadsFixture(t *testing.T) {
	t.Parallel()

	root := "../../tests/fixtures/store"

	snap, err := store.LocalSnapshot(root, store.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: "",
		SourceRef:        "",
		ResolvedCommit:   "",
		DefaultBranch:    "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := snap.Catalog["go"]; !ok {
		t.Fatal("expected go module in catalog")
	}

	if len(snap.Deps["eslint/node/fnm/pnpm"]) != 1 {
		t.Fatalf("unexpected deps: %#v", snap.Deps["eslint/node/fnm/pnpm"])
	}

	// internal/ has no Taskfile.yml of its own, so it is a namespace whose
	// children are catalogued under their full namespaced name.
	if _, ok := snap.Catalog["internal/skipfiles"]; !ok {
		t.Fatalf("expected namespaced module in catalog: %#v", snap.Catalog)
	}

	if _, ok := snap.Catalog["internal"]; ok {
		t.Fatal("namespace directory must not be catalogued as a module")
	}

	if snap.ModuleDir(
		"internal/skipfiles",
	) != filepath.Join(
		root,
		"taskfiles",
		"internal",
		"skipfiles",
	) {
		t.Fatalf("unexpected module dir: %s", snap.ModuleDir("internal/skipfiles"))
	}
}
