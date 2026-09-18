// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/testsupport"
	"github.com/task-otter/Taskotter/internal/testsupport/faults"
)

type (
	stubCloser struct {
		err error
	}
)

const (
	payloadText  = "payload"
	wantErrText  = "expected error"
	fileName     = "file.txt"
	unexpectText = "unexpected error: %v"
	tarBlockSize = 512
	errWantFmt   = "err = %v, want %v"
	baseName     = "base"
	subName      = "sub"
	escapePath   = "../escape"
	corruptTar   = "not a tar archive"
)

var errStub = errors.New("stub failure")

func lockArchiveTestState(t *testing.T) {
	t.Helper()
	t.Cleanup(testsupport.Lock())
}

// TestPreferErrKeepsPrimaryError verifies the primary error wins over the cleanup error.
func TestPreferErrKeepsPrimaryError(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	if !errors.Is(preferErr(errStub, io.EOF), errStub) {
		t.Fatal("primary error should win")
	}

	if !errors.Is(preferErr(nil, errStub), errStub) {
		t.Fatal("cleanup error should surface")
	}

	if preferErr(nil, nil) != nil {
		t.Fatal("expected nil")
	}
}

// TestCloseGzipReaderReportsFailure verifies a failing close is wrapped.
func TestCloseGzipReaderReportsFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	err := closeGzipReader(&stubCloser{err: errStub})

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}

	err = closeGzipReader(&stubCloser{err: nil})
	if err != nil {
		t.Fatalf(unexpectText, err)
	}
}

// TestDiscardTarEntryConsumesPayload verifies sized entries are drained and failures reported.
func TestDiscardTarEntryConsumesPayload(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	err := discardTarEntry(strings.NewReader(payloadText), int64(len(payloadText)))
	if err != nil {
		t.Fatalf(unexpectText, err)
	}

	err = discardTarEntry(&faults.StubReader{Err: nil}, int64(len(payloadText)))
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestAbsPathsReportFailures verifies both abs resolutions are reported when they fail.
func TestAbsPathsReportFailures(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)
	assertAbsPathsFail(t, consts.IndexZero)
	assertAbsPathsFail(t, consts.IndexOne)
}

// TestEnsureInsideRejectsEscape verifies targets outside the base are rejected.
func TestEnsureInsideRejectsEscape(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	base := t.TempDir()

	err := ensureInside(base, filepath.Join(base, "..", "escape"))
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestEnsureInsideReportsAbsFailure verifies an unresolvable path is rejected.
func TestEnsureInsideReportsAbsFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	err := ensureInsideWith(failingAbsPath, baseName, baseName+"/target")
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestOpenTargetReportsFailures verifies invalid modes and unopenable paths are reported.
func TestOpenTargetReportsFailures(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	file, err := openTarget(filepath.Join(t.TempDir(), "missing-dir", fileName), consts.FilePerm644)
	discardFile(file)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestSafeTarFileModeRejectsNegative verifies a negative mode is rejected.
func TestSafeTarFileModeRejectsNegative(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	mode, err := safeTarFileMode(-consts.IndexOne)
	iox.Discard(mode)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestValidTarPathRejectsUnsafeNames verifies absolute, traversing, and windows paths are rejected.
func TestValidTarPathRejectsUnsafeNames(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	unsafe := []string{"/etc/passwd", escapePath, `dir\file`}

	for i := range unsafe {
		if validTarPath(unsafe[i]) {
			t.Fatalf("validTarPath(%q) = true", unsafe[i])
		}
	}

	if !validTarPath("dir/file") {
		t.Fatal("validTarPath(dir/file) = false")
	}
}

// TestValidateCopySizeReportsMismatch verifies oversized and short reads are reported.
func TestValidateCopySizeReportsMismatch(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	err := validateCopySize(MaxFileBytes+consts.IndexOne, MaxFileBytes, fileName)
	if err == nil {
		t.Fatal(wantErrText)
	}

	err = validateCopySize(consts.IndexOne, consts.IndexTwo, fileName)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestWriteDirEntryReportsFailure verifies an unusable directory path is reported.
func TestWriteDirEntryReportsFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	blocker := filepath.Join(t.TempDir(), fileName)
	writeFileOrFail(t, blocker)

	err := writeDirEntryWithWrap(filepath.Join(blocker, subName))
	if err == nil {
		t.Fatal(wantErrText)
	}

	err = writeDirTarget(filepath.Join(blocker, subName))
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestCopyAndCloseReportsCopyFailure verifies a failing source is reported and the file closed.
func TestCopyAndCloseReportsCopyFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(&faults.StubReader{Err: nil}))
	path := filepath.Join(t.TempDir(), fileName)

	file, err := openTarget(path, consts.FilePerm644)
	if err != nil {
		t.Fatalf(unexpectText, err)
	}

	err = copyAndClose(
		extractor,
		&copyCloseArgs{file: file, size: int64(len(payloadText)), path: path},
	)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestCopyAndCloseReportsCloseFailure verifies closing an already closed file is reported.
func TestCopyAndCloseReportsCloseFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))
	path, file := closedTarget(t)

	err := copyAndClose(extractor, &copyCloseArgs{file: file, size: consts.IndexZero, path: path})
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestCopyToReportsTruncatedPayload verifies a truncated entry payload is reported.
func TestCopyToReportsTruncatedPayload(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := truncatedEntryExtractor(t)

	written, err := copyLim(
		extractor,
		&copyLimArgs{dst: io.Discard, size: int64(len(payloadText)), path: fileName},
	)
	iox.Discard(written)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestWriteRegularFileReportsCopyFailure verifies a truncated payload fails after opening.
func TestWriteRegularFileReportsCopyFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := truncatedEntryExtractor(t)

	err := writeRegularFile(extractor, &writeRegularArgs{
		path: filepath.Join(t.TempDir(), fileName),
		mode: consts.FilePerm644,
		size: int64(len(payloadText)),
	})
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestWriteRegEntryReportsParentFailure verifies an unusable parent directory is reported.
func TestWriteRegEntryReportsParentFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))
	blocker := filepath.Join(t.TempDir(), fileName)

	writeFileOrFail(t, blocker)

	err := writeRegTarget(extractor,
		testHeader(tar.TypeReg, fileName, consts.IndexZero),
		filepath.Join(blocker, subName, fileName),
	)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestCopyLimReportsSizeMismatch verifies a short entry is reported.
func TestCopyLimReportsSizeMismatch(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))

	written, err := copyLim(
		extractor,
		&copyLimArgs{dst: io.Discard, size: int64(len(payloadText)), path: fileName},
	)
	iox.Discard(written)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestNextHeaderReportsCorruptArchive verifies a malformed tar stream is reported.
func TestNextHeaderReportsCorruptArchive(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(corruptTar)))

	header, err := nextHeader(extractor)
	iox.Discard(header)

	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want read failure", err)
	}
}

// TestStepReportsHeaderFailure verifies step propagates header failures.
func TestStepReportsHeaderFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(corruptTar)))

	cont, err := step(extractor)

	if cont || err == nil {
		t.Fatalf("step() = %t, %v", cont, err)
	}
}

// TestSkipMetadataReportsFailure verifies an unreadable metadata payload is reported.
func TestSkipMetadataReportsFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(&faults.StubReader{Err: nil}))
	header := testHeader(tar.TypeXGlobalHeader, "pax", consts.IndexOne)

	assertSkipMetadataFails(t, extractor, header)
}

// TestValidateSizeReportsTotalLimit verifies the cumulative size limit is enforced.
func TestValidateSizeReportsTotalLimit(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))

	extractor.total = MaxTotalExtracted

	err := validateSize(extractor, consts.IndexOne, fileName)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestWriteRegEntryReportsSizeFailure verifies an oversized entry is rejected.
func TestWriteRegEntryReportsSizeFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))

	err := writeRegTarget(extractor,
		testHeader(tar.TypeReg, fileName, MaxFileBytes+consts.IndexOne),
		filepath.Join(t.TempDir(), fileName),
	)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestWriteRegularFileReportsOpenFailure verifies an unopenable target is reported.
func TestWriteRegularFileReportsOpenFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))

	err := writeRegularFile(extractor, &writeRegularArgs{
		path: filepath.Join(t.TempDir(), "missing", fileName),
		mode: consts.FilePerm644,
		size: consts.IndexZero,
	})
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestWriteResolvedEntryRejectsEscape verifies an escaping relative path is rejected.
func TestWriteResolvedEntryRejectsEscape(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))

	err := writeResolvedEntry(extractor,
		testHeader(tar.TypeReg, fileName, consts.IndexZero),
		escapePath,
	)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestWriteResolvedEntryReportsWriteFailure verifies a failing entry write is reported.
func TestWriteResolvedEntryReportsWriteFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	extractor := extractorWithReader(t, tar.NewReader(strings.NewReader(consts.Empty)))
	blocker := filepath.Join(extractor.destDir, fileName)

	writeFileOrFail(t, blocker)

	err := writeResolvedEntry(extractor,
		testHeader(tar.TypeDir, fileName, consts.IndexZero),
		fileName+"/"+subName,
	)
	if err == nil {
		t.Fatal(wantErrText)
	}
}

// TestExtractTarGzReportsDestinationFailure verifies an unusable destination is reported.
func TestExtractTarGzReportsDestinationFailure(t *testing.T) {
	t.Parallel()
	lockArchiveTestState(t)

	blocker := filepath.Join(t.TempDir(), fileName)
	writeFileOrFail(t, blocker)

	root, err := ExtractTarGz(bytes.NewReader(nil), filepath.Join(blocker, "dest"))
	iox.Discard(root)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func (closer *stubCloser) Close() error {
	return closer.err
}

func assertAbsPathsFail(t *testing.T, failAfter int) {
	t.Helper()

	base, target, err := absPathsWith(absPathFailureStub(failAfter), "base", "target")
	iox.Discard2(base, target)

	if !errors.Is(err, errStub) {
		t.Fatalf(errWantFmt, err, errStub)
	}
}

func absPathFailureStub(failAfter int) func(string) (string, error) {
	calls := consts.IndexZero

	return func(path string) (string, error) {
		defer func() { calls++ }()

		if calls >= failAfter {
			return consts.Empty, errStub
		}

		return filepath.Abs(path)
	}
}

func closedTarget(t *testing.T) (string, *os.File) {
	t.Helper()

	path := filepath.Join(t.TempDir(), fileName)

	file, err := openTarget(path, consts.FilePerm644)
	if err != nil {
		t.Fatalf(unexpectText, err)
	}

	iox.Discard(file.Close())

	return path, file
}

func assertSkipMetadataFails(t *testing.T, extractor *tarExtractor, header *tar.Header) {
	t.Helper()

	skip, err := shouldSkipEntry(extractor, header)
	iox.Discard(skip)

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func discardFile(file *os.File) {
	if file == nil {
		return
	}

	iox.Discard(file.Close())
}

func extractorWithReader(t *testing.T, reader *tar.Reader) *tarExtractor {
	t.Helper()

	return &tarExtractor{
		reader:     reader,
		destDir:    t.TempDir(),
		rootPrefix: consts.Empty,
		total:      consts.IndexZero,
		rootSet:    false,
	}
}

func failingAbsPath(string) (string, error) {
	return consts.Empty, errStub
}

// truncatedEntryExtractor returns an extractor whose current entry promises more
// bytes than the underlying stream holds.
func truncatedEntryExtractor(t *testing.T) *tarExtractor {
	t.Helper()

	truncated := singleEntryArchive(t)[:tarBlockSize]
	extractor := extractorWithReader(t, tar.NewReader(bytes.NewReader(truncated)))

	next, err := extractor.reader.Next()
	iox.Discard(next)
	failOnErr(t, err)

	return extractor
}

func singleEntryArchive(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	writer := tar.NewWriter(&buf)
	failOnErr(t, writer.WriteHeader(testHeader(tar.TypeReg, fileName, int64(len(payloadText)))))

	count, err := writer.Write([]byte(payloadText))
	iox.Discard(count)
	failOnErr(t, err)
	failOnErr(t, writer.Flush())

	return buf.Bytes()
}

func testHeader(typeflag byte, name string, size int64) *tar.Header {
	return &tar.Header{
		Typeflag: typeflag,
		Name:     name,
		Size:     size,
		Mode:     consts.FilePerm644,
	}
}

func failOnErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func writeFileOrFail(t *testing.T, path string) {
	t.Helper()

	err := os.WriteFile(path, []byte(payloadText), consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}
