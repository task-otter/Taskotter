// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package githubapi

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
)

const (
	ownerName   = "task-otter"
	repoName    = "Taskotter"
	branchHead  = "sync"
	branchBase  = "main"
	titleText   = "title"
	bodyText    = "body"
	badMethod   = "bad method"
	exampleURL  = "http://example.test/"
	prJSON      = `{"html_url":"https://example.test/pr/1","number":1}`
	prListJSON  = `[{"html_url":"https://example.test/pr/1","number":1}]`
	unexpectFmt = "%s: unexpected error: %v"
	wantErrFmt  = "%s: expected error"
	opNewClient = "NewClientWithHTTP"
	opEditPR    = "EditPRBody"
	opListPRs   = "ListOpenPRs"
	opNewReq    = "newAPIRequest"
)

var errStub = errors.New("stub failure")

// TestNewClientUsesDefaultBaseURL verifies the authenticated client targets api.github.com.
func TestNewClientUsesDefaultBaseURL(t *testing.T) {
	t.Parallel()

	client := NewClient(t.Context(), "token")

	if client.baseURL.Host != defaultAPIHost {
		t.Fatalf("baseURL = %q", client.baseURL.Host)
	}
}

// TestNewClientWithHTTPRejectsInvalidURL verifies an unparsable base URL is reported.
func TestNewClientWithHTTPRejectsInvalidURL(t *testing.T) {
	t.Parallel()

	client, err := NewClientWithHTTP("://invalid", http.DefaultClient)
	iox.Discard(client)

	if err == nil {
		t.Fatalf(wantErrFmt, opNewClient)
	}
}

// TestCreatePRReturnsPullRequest verifies a created pull request is decoded.
func TestCreatePRReturnsPullRequest(t *testing.T) {
	t.Parallel()

	client := newStubClient(t, http.StatusCreated, prJSON)

	pull, err := client.CreatePR(t.Context(), newCreateOptions())
	if err != nil {
		t.Fatalf(unexpectFmt, "CreatePR", err)
	}

	if pull.Number != consts.IndexOne {
		t.Fatalf("number = %d", pull.Number)
	}
}

// TestCreatePRReportsStatusError verifies a non-2xx response is reported.
func TestCreatePRReportsStatusError(t *testing.T) {
	t.Parallel()

	client := newStubClient(t, http.StatusInternalServerError, consts.Empty)

	pull, err := client.CreatePR(t.Context(), newCreateOptions())
	iox.Discard(pull)

	if !errors.Is(err, errGitHubAPIStatus) {
		t.Fatalf("err = %v, want status error", err)
	}
}

// TestEditPRBodyIgnoresResponseBody verifies a nil destination skips decoding.
func TestEditPRBodyIgnoresResponseBody(t *testing.T) {
	t.Parallel()

	client := newStubClient(t, http.StatusOK, prJSON)

	err := client.EditPRBody(t.Context(), newEditOptions())
	if err != nil {
		t.Fatalf(unexpectFmt, opEditPR, err)
	}
}

// TestEditPRBodyReportsStatusError verifies a failed edit surfaces the status error.
func TestEditPRBodyReportsStatusError(t *testing.T) {
	t.Parallel()

	client := newStubClient(t, http.StatusNotFound, consts.Empty)

	err := client.EditPRBody(t.Context(), newEditOptions())
	if err == nil {
		t.Fatalf(wantErrFmt, opEditPR)
	}
}

// TestListOpenPRsDecodesResults verifies query building and list decoding.
func TestListOpenPRsDecodesResults(t *testing.T) {
	t.Parallel()

	client := newStubClient(t, http.StatusOK, prListJSON)

	prs, err := client.ListOpenPRs(t.Context(), newListOptions())
	if err != nil {
		t.Fatalf(unexpectFmt, opListPRs, err)
	}

	if len(prs) != consts.IndexOne {
		t.Fatalf("prs = %d", len(prs))
	}
}

// TestListOpenPRsReportsDecodeError verifies malformed JSON is reported.
func TestListOpenPRsReportsDecodeError(t *testing.T) {
	t.Parallel()

	client := newStubClient(t, http.StatusOK, "not json")

	prs, err := client.ListOpenPRs(t.Context(), newListOptions())
	iox.Discard(prs)

	if err == nil {
		t.Fatalf(wantErrFmt, opListPRs)
	}
}

// TestListOpenPRsReportsTransportError verifies transport failures are wrapped.
func TestListOpenPRsReportsTransportError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(handleStub(http.StatusOK, consts.Empty))
	client := clientFor(t, server.URL)

	server.Close()

	prs, err := client.ListOpenPRs(t.Context(), newListOptions())
	iox.Discard(prs)

	if err == nil {
		t.Fatalf(wantErrFmt, opListPRs)
	}
}

// TestMarshalPayloadReportsError verifies unencodable payloads are reported.
func TestMarshalPayloadReportsError(t *testing.T) {
	t.Parallel()

	data, err := marshalPayload(make(chan int))
	iox.Discard(data)

	if err == nil {
		t.Fatalf(wantErrFmt, "marshalPayload")
	}
}

// TestNewAPIRequestReportsError verifies an invalid method is reported.
func TestNewAPIRequestReportsError(t *testing.T) {
	t.Parallel()
	assertNewAPIRequestFails(t, newCall(badMethod, nil))
}

// TestNewAPIRequestReportsMarshalError verifies an unencodable payload aborts request building.
func TestNewAPIRequestReportsMarshalError(t *testing.T) {
	t.Parallel()
	assertNewAPIRequestFails(t, newCall(http.MethodPost, make(chan int)))
}

// TestDoRequestReportsBuildError verifies request-building failures are wrapped.
func TestDoRequestReportsBuildError(t *testing.T) {
	t.Parallel()

	client := newStubClient(t, http.StatusOK, consts.Empty)

	//nolint:bodyclose // the request never leaves the client, so no response body exists
	resp, err := client.doRequest(t.Context(), newCall(badMethod, nil))
	iox.Discard(resp)

	if err == nil {
		t.Fatalf(wantErrFmt, "doRequest")
	}
}

// TestAppendBodyCloseReportsFailures verifies drain and close failures are reported.
func TestAppendBodyCloseReportsFailures(t *testing.T) {
	t.Parallel()

	cases := []*stubBody{
		{reader: strings.NewReader(consts.Empty), readErr: errStub, closeErr: nil},
		{reader: strings.NewReader(consts.Empty), readErr: nil, closeErr: errStub},
	}

	for i := range cases {
		//nolint:bodyclose // appendBodyClose closes the body under test
		err := appendBodyClose(nil, newStubResponse(cases[i]))
		if err == nil {
			t.Fatalf(wantErrFmt, "appendBodyClose")
		}
	}
}

// TestAppendBodyCloseKeepsExistingError verifies an earlier error wins over cleanup errors.
func TestAppendBodyCloseKeepsExistingError(t *testing.T) {
	t.Parallel()

	body := &stubBody{reader: strings.NewReader(consts.Empty), readErr: errStub, closeErr: errStub}

	//nolint:bodyclose // appendBodyClose closes the body under test
	err := appendBodyClose(errStub, newStubResponse(body))

	if !errors.Is(err, errStub) {
		t.Fatalf("err = %v, want %v", err, errStub)
	}
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

func assertNewAPIRequestFails(t *testing.T, call *jsonCall) {
	t.Helper()

	req, err := newAPIRequest(t.Context(), call, exampleURL)
	iox.Discard(req)

	if err == nil {
		t.Fatalf(wantErrFmt, opNewReq)
	}
}

func clientFor(t *testing.T, baseURL string) *Client {
	t.Helper()

	client, err := NewClientWithHTTP(baseURL, http.DefaultClient)
	if err != nil {
		t.Fatalf(unexpectFmt, opNewClient, err)
	}

	return client
}

func handleStub(status int, body string) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		iox.Discard(req.Method)
		writer.WriteHeader(status)
		iox.FprintBestEffort(writer, body)
	}
}

func newCall(method string, payload any) *jsonCall {
	return &jsonCall{method: method, path: consts.PathSepString, payload: payload, dest: nil}
}

func newCreateOptions() *CreatePROptions {
	return &CreatePROptions{
		Owner: ownerName,
		Repo:  repoName,
		Title: titleText,
		Head:  branchHead,
		Base:  branchBase,
		Body:  bodyText,
	}
}

func newEditOptions() *EditPRBodyOptions {
	return &EditPRBodyOptions{
		Owner:  ownerName,
		Repo:   repoName,
		Body:   bodyText,
		Number: consts.IndexOne,
	}
}

func newListOptions() *ListOpenPROptions {
	return &ListOpenPROptions{
		Owner: ownerName,
		Repo:  repoName,
		Head:  branchHead,
		Base:  branchBase,
	}
}

func newStubClient(t *testing.T, status int, body string) *Client {
	t.Helper()

	server := httptest.NewServer(handleStub(status, body))
	t.Cleanup(server.Close)

	return clientFor(t, server.URL)
}

func newStubResponse(body io.ReadCloser) *http.Response {
	return &http.Response{Body: body} //nolint:exhaustruct_v5 // appendBodyClose only reads the body
}
