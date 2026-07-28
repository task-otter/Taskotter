package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogithub "github.com/google/go-github/v69/github"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/syncer"
)

const (
	outputKeyChanged = "changed"
	outputFalse      = "false"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()

	srv := httptest.NewServer(handler)
	api := gogithub.NewClient(srv.Client())

	baseURL, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	api.BaseURL = baseURL
	api.UploadURL = baseURL

	return &Client{api: api, owner: "owner", repo: "repo"}, srv.Close
}

func TestNewClientParsesRepository(t *testing.T) {
	t.Parallel()

	client, err := NewClient(context.Background(), "token", "owner/repo")
	if err != nil {
		t.Fatal(err)
	}

	if client.owner != "owner" || client.repo != "repo" {
		t.Fatalf("NewClient() = %#v", client)
	}
}

func TestNewClientRejectsInvalidRepository(t *testing.T) {
	t.Parallel()

	_, err := NewClient(context.Background(), "token", "owner")
	if err == nil {
		t.Fatal("expected invalid repository error")
	}
}

func TestFindOpenPR(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/repos/owner/repo/pulls" {
			http.NotFound(writer, req)

			return
		}

		if req.URL.Query().Get("head") != "owner:taskotter/sync-abc" {
			t.Fatalf("head query = %q", req.URL.Query().Get("head"))
		}

		if req.URL.Query().Get("base") != "main" {
			t.Fatalf("base query = %q", req.URL.Query().Get("base"))
		}

		_, _ = writer.Write([]byte(`[{"number":42,"html_url":"https://example.test/pr/42"}]`))
	})
	defer cleanup()

	pullRequest, err := client.FindOpenPR(context.Background(), "taskotter/sync-abc", "main")
	if err != nil {
		t.Fatal(err)
	}

	if pullRequest.Number != 42 || pullRequest.URL != "https://example.test/pr/42" {
		t.Fatalf("FindOpenPR() = %#v", pullRequest)
	}
}

func TestFindOpenPRNotFound(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[]`))
	})
	defer cleanup()

	_, err := client.FindOpenPR(context.Background(), "taskotter/sync-abc", "main")
	if !errors.Is(err, ErrPullRequestNotFound) {
		t.Fatalf("FindOpenPR() error = %v, want ErrPullRequestNotFound", err)
	}
}

func TestFindOpenPRError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "nope", http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := client.FindOpenPR(context.Background(), "taskotter/sync-abc", "main")
	if err == nil {
		t.Fatal("expected FindOpenPR error")
	}
}

func TestCreatePR(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/repos/owner/repo/pulls" {
			http.NotFound(writer, req)

			return
		}

		_, _ = writer.Write([]byte(`{"number":7,"html_url":"https://example.test/pr/7"}`))
	})
	defer cleanup()

	pullRequest, err := client.CreatePR(context.Background(), "taskotter/sync-abc", "main", "body")
	if err != nil {
		t.Fatal(err)
	}

	if pullRequest.Number != 7 || pullRequest.URL != "https://example.test/pr/7" {
		t.Fatalf("CreatePR() = %#v", pullRequest)
	}
}

func TestCreatePRError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "nope", http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := client.CreatePR(context.Background(), "taskotter/sync-abc", "main", "body")
	if err == nil {
		t.Fatal("expected CreatePR error")
	}
}

func TestUpdatePRBody(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/repos/owner/repo/pulls/7" {
			http.NotFound(writer, req)

			return
		}

		_, _ = writer.Write([]byte(`{"number":7}`))
	})
	defer cleanup()

	err := client.UpdatePRBody(context.Background(), 7, "body")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdatePRBodyError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "nope", http.StatusInternalServerError)
	})
	defer cleanup()

	err := client.UpdatePRBody(context.Background(), 7, "body")
	if err == nil {
		t.Fatal("expected UpdatePRBody error")
	}
}

func TestBuildPRBodyWithoutNodeRuntimeIncludesFileLists(t *testing.T) {
	t.Parallel()

	cfg := new(config.Config)
	cfg.Tasks = []string{"go"}
	cfg.IncludesDoc = false
	cfg.SyncRoot = false
	cfg.TargetFolder = "taskfiles"
	cfg.StoreVersion = "v1.2.3"

	plan := new(syncer.Plan)
	plan.Requested = map[string]syncer.ModuleRecord{
		"go": {
			SourceModule:      "go",
			DestinationModule: "",
			Path:              "taskfiles/go",
		},
	}
	plan.Added = []string{"taskfiles/go/Taskfile.yml"}
	plan.Updated = []string{"taskfiles/Taskfile.yml"}
	plan.Removed = []string{"taskfiles/old.yml"}

	body := BuildPRBody(cfg, plan, StoreRef{
		SourceRef:      "",
		ResolvedCommit: "",
		DefaultBranch:  "",
	})
	if strings.Contains(body, "Package manager") {
		t.Fatalf("non-node body should not include package manager: %s", body)
	}

	for _, want := range []string{"v1.2.3", "taskfiles/go/Taskfile.yml", "taskfiles/old.yml"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestWriteOutputs(t *testing.T) {
	t.Parallel()

	err := WriteOutputs("", map[string]string{outputKeyChanged: outputFalse})
	if err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(t.TempDir(), "missing", "output")

	err = WriteOutputs(outputPath, map[string]string{outputKeyChanged: outputFalse})
	if err == nil {
		t.Fatal("expected write error")
	}

	outputPath = filepath.Join(t.TempDir(), "output")

	err = WriteOutputs(outputPath, map[string]string{outputKeyChanged: outputFalse})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "changed=false\n" {
		t.Fatalf("output = %q", data)
	}
}
