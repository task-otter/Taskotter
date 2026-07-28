package store_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/store"
)

func TestResolveRefDefaultBranch(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/repos/mostafakhairy0305-dot/TaskOtter-store":
			_, _ = writer.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/mostafakhairy0305-dot/TaskOtter-store/commits/main":
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
		case "/repos/mostafakhairy0305-dot/TaskOtter-store":
			_, _ = writer.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/mostafakhairy0305-dot/TaskOtter-store/git/ref/tags/v9.9.9":
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
