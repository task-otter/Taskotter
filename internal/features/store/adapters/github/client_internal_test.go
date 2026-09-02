// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	stubBody struct {
		reader   io.Reader
		readErr  error
		closeErr error
	}

	stubDoer struct {
		err error
	}

	// scriptedDoer answers requests by path suffix and fails for anything else.
	scriptedDoer struct {
		responses map[string]string
	}
)

const (
	wantErrText = "expected error"
	testTag     = "v1.2.3"
	testBranch  = "main"
	testSHA     = "abc123"
	errFmt      = "err = %v"
	badArchive  = "not a gzip archive"
	childName   = "child"
)

var errStub = errors.New("stub failure")

// TestLocalSnapshotReportsLoadFailure verifies a missing store root is reported.
func TestLocalSnapshotReportsLoadFailure(t *testing.T) {
	t.Parallel()

	snapshot, err := LocalSnapshot(t.TempDir(), newRefInfoForTest())
	iox.Discard(snapshot)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestClientRequestsReportTransportFailures verifies every request path wraps doer errors.
func TestClientRequestsReportTransportFailures(t *testing.T) {
	t.Parallel()

	client := NewClientWithHTTP(t.Context(), consts.Empty, &stubDoer{err: errStub})
	ctx := t.Context()

	assertFails(t, secondOf(client.resolveBranchHead(ctx, testBranch)))
	assertFails(t, secondOf(client.resolveTag(ctx, testTag)))
	assertFails(t, secondOf(client.peelAnnotatedTag(ctx, testSHA)))
	assertFails(t, secondOf(client.ResolveRef(ctx, consts.Empty)))
	assertFails(t, secondOf(client.DownloadSnapshot(ctx, newRefInfoForTest())))
}

// TestResolveVersionRefReportsTagFailure verifies a requested tag failure is wrapped.
func TestResolveVersionRefReportsTagFailure(t *testing.T) {
	t.Parallel()

	client := NewClientWithHTTP(t.Context(), consts.Empty, &stubDoer{err: errStub})
	info := newRefInfo(testTag, testBranch)

	err := client.resolveVersionRef(t.Context(), &versionRefRequest{
		info:             &info,
		requestedVersion: testTag,
		defaultBranch:    testBranch,
	})
	assertFails(t, err)
}

// TestResolveRefReportsBranchHeadFailure verifies a failing branch lookup is wrapped
// after the repository metadata request succeeds.
func TestResolveRefReportsBranchHeadFailure(t *testing.T) {
	t.Parallel()

	doer := &scriptedDoer{responses: map[string]string{
		"/store": `{"default_branch":"main"}`,
	}}
	client := NewClientWithHTTP(t.Context(), consts.Empty, doer)

	assertFails(t, secondOf(client.ResolveRef(t.Context(), consts.Empty)))
}

// TestTagSHAReportsPeelFailure verifies an annotated tag whose peel request fails is reported.
func TestTagSHAReportsPeelFailure(t *testing.T) {
	t.Parallel()

	doer := &scriptedDoer{responses: map[string]string{
		"/git/ref/tags/" + testTag: `{"object":{"sha":"abc123","type":"tag"}}`,
	}}
	client := NewClientWithHTTP(t.Context(), consts.Empty, doer)

	assertFails(t, secondOf(client.resolveTag(t.Context(), testTag)))
}

// TestBuildSnapshotReportsTempDirFailure verifies an unusable temp directory is reported.
func TestBuildSnapshotReportsTempDirFailure(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))

	snapshot, err := buildSnapshot(strings.NewReader(consts.Empty), newRefInfoForTest())
	iox.Discard(snapshot)
	assertFails(t, err)
}

// TestDoGetReportsInvalidURL verifies an unbuildable request is reported.
func TestDoGetReportsInvalidURL(t *testing.T) {
	t.Parallel()

	client := NewClientWithHTTP(t.Context(), "token", &stubDoer{err: errStub}).WithBaseURL("://bad")

	//nolint:bodyclose // the request is never sent, so there is no body
	resp, err := client.doGet(t.Context(), "\n")
	iox.Discard(resp)
	assertFails(t, err)
}

// TestExtractDefaultBranchRejectsBadPayloads verifies malformed and empty branches fail.
func TestExtractDefaultBranchRejectsBadPayloads(t *testing.T) {
	t.Parallel()

	branch, err := extractDefaultBranch(rawPayload(`{"default_branch":5}`))
	iox.Discard(branch)
	assertFails(t, err)

	branch, err = extractDefaultBranch(rawPayload(`{"default_branch":""}`))
	iox.Discard(branch)

	if !errors.Is(err, errDefaultBranchEmpty) {
		t.Fatalf(errFmt, err)
	}
}

// TestExtractAndLoadReportsFailures verifies archive and catalog failures are reported.
func TestExtractAndLoadReportsFailures(t *testing.T) {
	t.Parallel()

	extracted, err := extractAndLoad(strings.NewReader(badArchive), t.TempDir())
	iox.Discard(extracted)
	assertFails(t, err)

	extracted, err = extractAndLoad(emptyArchive(t), t.TempDir())
	iox.Discard(extracted)
	assertFails(t, err)
}

// TestBuildSnapshotCleansUpAfterFailure verifies the temp directory is removed on failure.
func TestBuildSnapshotCleansUpAfterFailure(t *testing.T) {
	t.Parallel()

	snapshot, err := buildSnapshot(strings.NewReader(badArchive), newRefInfoForTest())
	iox.Discard(snapshot)
	assertFails(t, err)
}

// TestCleanupTempDirAfterErrorJoinsRemoveFailure verifies both errors are reported.
func TestCleanupTempDirAfterErrorJoinsRemoveFailure(t *testing.T) {
	t.Parallel()

	err := cleanupTempDirAfterError(filepath.Join(unremovableDir(t), childName), errStub)

	if !errors.Is(err, errStub) {
		t.Fatalf(errFmt, err)
	}
}

// TestSnapshotCleanupReportsRemoveFailure verifies cleanup failures are reported.
func TestSnapshotCleanupReportsRemoveFailure(t *testing.T) {
	t.Parallel()
	assertFails(t, snapshotCleanup(filepath.Join(unremovableDir(t), childName))())
}

// TestDrainResponseBodyReportsReadFailure verifies unreadable bodies are reported.
//
//nolint:bodyclose // the stub responses are drained by the function under test
func TestDrainResponseBodyReportsReadFailure(t *testing.T) {
	t.Parallel()
	assertFails(t, drainResponseBody(failingResponse()))
}

// TestDrainArchiveBodyReportsFailures verifies read and close failures are reported.
//
//nolint:bodyclose // the stub responses are drained by the function under test
func TestDrainArchiveBodyReportsFailures(t *testing.T) {
	t.Parallel()

	assertFails(t, secondOf(drainArchiveBody(failingResponse())))
	assertFails(t, secondOf(drainArchiveBody(closeFailingResponse())))
}

// TestCloseOnArchiveStatusErrorReportsCleanupFailures verifies drain and close failures join.
//
//nolint:bodyclose // the stub responses are drained by the function under test
func TestCloseOnArchiveStatusErrorReportsCleanupFailures(t *testing.T) {
	t.Parallel()

	assertFails(t, closeOnArchiveStatusError(notFoundResponse(failingBody())))
	assertFails(t, closeOnArchiveStatusError(notFoundResponse(closeFailingBody())))
	assertFails(t, closeOnArchiveStatusError(notFoundResponse(okBody())))
}

// TestResolveSHAReportsPeelFailure verifies annotated tag peeling failures are reported.
func TestResolveSHAReportsPeelFailure(t *testing.T) {
	t.Parallel()

	client := NewClientWithHTTP(t.Context(), consts.Empty, &stubDoer{err: errStub})
	payload := annotatedTagRefPayload()

	sha, err := client.resolveSHA(t.Context(), &payload)
	iox.Discard(sha)
	assertFails(t, err)
}

func (body *stubBody) Close() error {
	return body.closeErr
}

func (body *stubBody) Read(data []byte) (int, error) {
	if body.readErr != nil {
		return len(data) * consts.IndexZero, body.readErr
	}

	//nolint:wrapcheck // the io.Reader contract requires the unwrapped io.EOF sentinel
	return body.reader.Read(data)
}

func (doer *stubDoer) Do(*http.Request) (*http.Response, error) {
	return nil, doer.err
}

func (doer *scriptedDoer) Do(req *http.Request) (*http.Response, error) {
	for suffix := range doer.responses {
		if strings.HasSuffix(req.URL.Path, suffix) {
			return newResponse(http.StatusOK, okBodyWith(doer.responses[suffix])), nil
		}
	}

	return nil, errStub
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func annotatedTagRefPayload() tagRefPayload {
	var payload tagRefPayload

	payload.Object.Type = gitObjectTypeTag
	payload.Object.SHA = testSHA

	return payload
}

func closeFailingBody() *stubBody {
	return &stubBody{reader: strings.NewReader(consts.Empty), readErr: nil, closeErr: errStub}
}

func closeFailingResponse() *http.Response {
	return newResponse(http.StatusOK, closeFailingBody())
}

// emptyArchive returns a valid gzip stream holding an empty tar, which extracts
// to a store root without a taskfiles tree.
func emptyArchive(t *testing.T) *bytes.Reader {
	t.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	failOnErr(t, tarWriter.Close())
	failOnErr(t, gzipWriter.Close())

	return bytes.NewReader(buf.Bytes())
}

func failOnErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func failingBody() *stubBody {
	return &stubBody{reader: strings.NewReader(consts.Empty), readErr: errStub, closeErr: nil}
}

func failingResponse() *http.Response {
	return newResponse(http.StatusOK, failingBody())
}

func notFoundResponse(body io.ReadCloser) *http.Response {
	return newResponse(http.StatusNotFound, body)
}

func newRefInfoForTest() *RefInfo {
	info := newRefInfo(consts.Empty, testBranch)

	return &info
}

func newResponse(status int, body io.ReadCloser) *http.Response {
	//nolint:exhaustruct_v5 // only the status and body are read
	return &http.Response{StatusCode: status, Body: body}
}

func okBody() *stubBody {
	return okBodyWith(consts.Empty)
}

func okBodyWith(body string) *stubBody {
	return &stubBody{reader: strings.NewReader(body), readErr: nil, closeErr: nil}
}

func rawPayload(body string) map[string]json.RawMessage {
	payload := make(map[string]json.RawMessage)
	iox.Discard(json.Unmarshal([]byte(body), &payload))

	return payload
}

func secondOf[T any](_ T, err error) error {
	return err
}

// unremovableDir returns a directory whose children cannot be removed.
func unremovableDir(t *testing.T) string {
	t.Helper()

	if os.Geteuid() == consts.IndexZero {
		t.Skip("running as root: permission errors are not reproducible")
	}

	blocked := filepath.Join(t.TempDir(), "blocked")

	failOnErr(t, os.MkdirAll(filepath.Join(blocked, childName), consts.FilePerm755))
	failOnErr(t, os.Chmod(blocked, consts.IndexZero))
	t.Cleanup(func() { iox.Discard(os.Chmod(blocked, consts.FilePerm755)) })

	return blocked
}
