// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
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
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/syncer"
)

const (
	outputKeyChanged     = "changed"
	outputFalse          = "false"
	errBodyNope          = "nope"
	testSyncBranch       = "taskotter/sync-abc"
	testMainBranch       = "main"
	testPRBody           = "body"
	testStoreVersionV123 = "v1.2.3"
	testAddedTaskfile    = "taskfiles/go/Taskfile.yml"
	testRemovedTaskfile  = "taskfiles/old.yml"
	dirOutput            = "output"
	testOwner            = "owner"
	testRepoName         = "repo"
	testToken            = "token"
	pathReposPulls       = "/repos/owner/repo/pulls"
	queryHead            = "head"
	queryBase            = "base"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (client *Client, cleanup func()) {
	t.Helper()

	srv := httptest.NewServer(handler)
	api := gogithub.NewClient(srv.Client())

	baseURL, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	api.BaseURL = baseURL
	api.UploadURL = baseURL

	return &Client{api: api, owner: testOwner, repo: testRepoName}, srv.Close
}

// TestNewClientParsesRepository verifies the owner and repo name are parsed from the coordinate.
func TestNewClientParsesRepository(t *testing.T) {
	t.Parallel()

	client, err := NewClient(t.Context(), testToken, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}

	if client.owner != testOwner || client.repo != testRepoName {
		t.Fatalf("NewClient() = %#v", client)
	}
}

// TestNewClientRejectsInvalidRepository verifies an invalid repository coordinate returns an error.
func TestNewClientRejectsInvalidRepository(t *testing.T) {
	t.Parallel()

	_, err := NewClient(t.Context(), testToken, testOwner)
	if err == nil {
		t.Fatal("expected invalid repository error")
	}
}

func assertOpenPRQuery(t *testing.T, req *http.Request) {
	t.Helper()

	if req.URL.Query().Get(queryHead) != "owner:taskotter/sync-abc" {
		t.Fatalf("head query = %q", req.URL.Query().Get(queryHead))
	}

	if req.URL.Query().Get(queryBase) != testMainBranch {
		t.Fatalf("base query = %q", req.URL.Query().Get(queryBase))
	}
}

func openPRHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != pathReposPulls {
			http.NotFound(writer, req)

			return
		}

		assertOpenPRQuery(t, req)

		_, _ = writer.Write(
			[]byte(`[{"number":42,"html_url":"https://example.test/pr/42"}]`),
		)
	}
}

// TestFindOpenPR verifies an existing open pull request is found by branch and base.
func TestFindOpenPR(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, openPRHandler(t))

	defer cleanup()

	pullRequest, err := client.FindOpenPR(t.Context(), testSyncBranch, testMainBranch)
	if err != nil {
		t.Fatal(err)
	}

	if pullRequest.Number != consts.Index42 || pullRequest.URL != "https://example.test/pr/42" {
		t.Fatalf("FindOpenPR() = %#v", pullRequest)
	}
}

// TestFindOpenPRNotFound verifies ErrPullRequestNotFound is returned when no PR matches.
func TestFindOpenPRNotFound(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[]`)) //nolint:errcheck // test stub write cannot fail
	})

	defer cleanup()

	_, err := client.FindOpenPR(t.Context(), testSyncBranch, testMainBranch)

	if !errors.Is(err, ErrPullRequestNotFound) {
		t.Fatalf("FindOpenPR() error = %v, want ErrPullRequestNotFound", err)
	}
}

// TestFindOpenPRError verifies a server error response is surfaced as an error.
func TestFindOpenPRError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, errBodyNope, http.StatusInternalServerError)
	})

	defer cleanup()

	_, err := client.FindOpenPR(t.Context(), testSyncBranch, testMainBranch)
	if err == nil {
		t.Fatal("expected FindOpenPR error")
	}
}

func createPRHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != pathReposPulls {
			http.NotFound(writer, req)

			return
		}

		_, _ = writer.Write(
			[]byte(`{"number":7,"html_url":"https://example.test/pr/7"}`),
		)
	}
}

// TestCreatePR verifies a new pull request is created and its number and URL returned.
func TestCreatePR(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, createPRHandler())

	defer cleanup()

	pullRequest, err := client.CreatePR(t.Context(), testSyncBranch, testMainBranch, testPRBody)
	if err != nil {
		t.Fatal(err)
	}

	if pullRequest.Number != consts.IndexSeven || pullRequest.URL != "https://example.test/pr/7" {
		t.Fatalf("CreatePR() = %#v", pullRequest)
	}
}

// TestCreatePRError verifies a server error response during PR creation is surfaced.
func TestCreatePRError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, errBodyNope, http.StatusInternalServerError)
	})

	defer cleanup()

	_, err := client.CreatePR(t.Context(), testSyncBranch, testMainBranch, testPRBody)
	if err == nil {
		t.Fatal("expected CreatePR error")
	}
}

// TestUpdatePRBody verifies the pull request body is updated via a PATCH request.
func TestUpdatePRBody(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch || req.URL.Path != "/repos/owner/repo/pulls/7" {
			http.NotFound(writer, req)

			return
		}

		_, _ = writer.Write([]byte(`{"number":7}`)) //nolint:errcheck // test stub write cannot fail
	})

	defer cleanup()

	err := client.UpdatePRBody(t.Context(), consts.IndexSeven, testPRBody)
	if err != nil {
		t.Fatal(err)
	}
}

// TestUpdatePRBodyError verifies a server error response during body update is surfaced.
func TestUpdatePRBodyError(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, errBodyNope, http.StatusInternalServerError)
	})

	defer cleanup()

	err := client.UpdatePRBody(t.Context(), consts.IndexSeven, testPRBody)
	if err == nil {
		t.Fatal("expected UpdatePRBody error")
	}
}

func buildNonNodeConfig() *config.Config {
	cfg := new(config.Config)

	cfg.Tasks = []string{consts.Go}
	cfg.IncludesDoc = false
	cfg.SyncRoot = false
	cfg.TargetFolder = "taskfiles"
	cfg.StoreVersion = testStoreVersionV123

	return cfg
}

func buildNonNodeTestPlanData() *syncer.Plan {
	plan := new(syncer.Plan)

	plan.Requested = map[string]syncer.ModuleRecord{
		consts.Go: {
			SourceModule:      consts.Go,
			DestinationModule: consts.Empty,
			Path:              "taskfiles/go",
		},
	}
	plan.Added = []string{testAddedTaskfile}
	plan.Updated = []string{"taskfiles/Taskfile.yml"}
	plan.Removed = []string{testRemovedTaskfile}

	return plan
}

func assertBodyContainsAll(t *testing.T, body string, wants []string) {
	t.Helper()

	for i := range wants {
		if !strings.Contains(body, wants[i]) {
			t.Fatalf("body missing %q: %s", wants[i], body)
		}
	}
}

// TestBuildPRBodyWithoutNodeRuntimeIncludesFileLists verifies non-node bodies list files without package manager info.
func TestBuildPRBodyWithoutNodeRuntimeIncludesFileLists(t *testing.T) {
	t.Parallel()

	cfg := buildNonNodeConfig()
	plan := buildNonNodeTestPlanData()

	body := BuildPRBody(cfg, plan, &StoreRef{
		SourceRef:      consts.Empty,
		ResolvedCommit: consts.Empty,
		DefaultBranch:  consts.Empty,
	})

	if strings.Contains(body, "Package manager") {
		t.Fatalf("non-node body should not include package manager: %s", body)
	}

	assertBodyContainsAll(
		t,
		body,
		[]string{testStoreVersionV123, testAddedTaskfile, testRemovedTaskfile},
	)
}

func assertWriteOutputsFails(t *testing.T, outputPath string) {
	t.Helper()

	err := WriteOutputs(outputPath, map[string]string{outputKeyChanged: outputFalse})
	if err == nil {
		t.Fatal("expected write error")
	}
}

func assertWriteOutputsSucceeds(t *testing.T, outputPath string) {
	t.Helper()

	err := WriteOutputs(outputPath, map[string]string{outputKeyChanged: outputFalse})
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

// TestWriteOutputs verifies outputs write successfully or fail for an invalid path.
func TestWriteOutputs(t *testing.T) {
	t.Parallel()

	err := WriteOutputs(consts.Empty, map[string]string{outputKeyChanged: outputFalse})
	if err != nil {
		t.Fatal(err)
	}

	assertWriteOutputsFails(t, filepath.Join(t.TempDir(), "missing", dirOutput))
	assertWriteOutputsSucceeds(t, filepath.Join(t.TempDir(), dirOutput))
}
