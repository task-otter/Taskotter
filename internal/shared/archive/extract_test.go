// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/task-otter/Taskotter/internal/shared/archive"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	tarGzBuilder struct {
		t          *testing.T
		gzipWriter *gzip.Writer
		tarWriter  *tar.Writer
		buf        bytes.Buffer
	}

	tarEntrySpec struct {
		name    string
		content []byte
		mode    int64
	}
)

const (
	testRepoTaskfileGo = "repo-main/taskfiles/go/Taskfile.yml"

	testTaskfileGoPath = "taskfiles/go/Taskfile.yml"

	testContentX = "x"

	testContentNope = "nope"

	testContentTaskfileV3 = "version: \"3\"\n"
)

// TestExtractDirectoryEntry verifies directory entries are created on disk.
func TestExtractDirectoryEntry(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithHeaders(t, []*tar.Header{
		{Name: "repo-main/", Mode: consts.FilePerm755, Typeflag: tar.TypeDir},
		{Name: "repo-main/taskfiles", Mode: consts.FilePerm755, Typeflag: tar.TypeDir},
		regularTarHeader(testRepoTaskfileGo, int64(len(testContentTaskfileV3))),
	})
	dest := extractToTemp(t, data)

	if !extractedIsDir(t, dest, "taskfiles") {
		t.Fatal("taskfiles should be a directory")
	}
}

// TestExtractStripsSetuidMode verifies the setuid bit is stripped from extracted file modes.
func TestExtractStripsSetuidMode(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithMode(t, &tarEntrySpec{
		name:    testRepoTaskfileGo,
		content: []byte(testContentTaskfileV3),
		mode:    consts.FilePerm4755,
	})
	dest := extractToTemp(t, data)
	perm := extractedFilePerm(t, dest, testTaskfileGoPath)

	if perm != consts.FilePerm755 {
		t.Fatalf("file mode = %o, want %o", perm, consts.FilePerm755)
	}
}

// TestExtractValidArchive verifies a well-formed archive extracts its files successfully.
func TestExtractValidArchive(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		testRepoTaskfileGo: []byte(testContentTaskfileV3),
	})
	dest := extractToTemp(t, data)

	result, err := os.Stat(filepath.Join(dest, testTaskfileGoPath))
	iox.Discard(result)

	if err != nil {
		t.Fatal(err)
	}
}

// TestRejectBackslashPath verifies entry names containing backslashes are rejected.
func TestRejectBackslashPath(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		`repo-main\taskfiles\go\Taskfile.yml`: []byte(testContentNope),
	})
	dest := t.TempDir()

	result, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected backslash path rejection")
	}
}

// TestRejectFileLargerThanLimit verifies files exceeding the size limit are rejected.
func TestRejectFileLargerThanLimit(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithHeaders(t, []*tar.Header{
		regularTarHeader("repo-main/huge.bin", archive.MaxFileBytes+1),
	})

	result, err := archive.ExtractTarGz(bytes.NewReader(data), t.TempDir())
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected file size limit error")
	}
}

// TestRejectInvalidGzip verifies non-gzip input is rejected with an error.
func TestRejectInvalidGzip(t *testing.T) {
	t.Parallel()

	result, err := archive.ExtractTarGz(strings.NewReader("not gzip"), t.TempDir())
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected invalid gzip error")
	}
}

// TestRejectNegativeFileMode verifies a negative tar file mode is rejected.
func TestRejectNegativeFileMode(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithMode(t, &tarEntrySpec{
		name:    testRepoTaskfileGo,
		content: []byte(testContentX),
		mode:    -1,
	})
	dest := t.TempDir()

	result, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected negative file mode rejection")
	}
}

// TestRejectSymlink verifies symlink entries are rejected during extraction.
func TestRejectSymlink(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithHeaders(t, []*tar.Header{
		symlinkTarHeader("repo-main/link", "/etc/passwd"),
	})

	result, err := archive.ExtractTarGz(bytes.NewReader(data), t.TempDir())
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected symlink rejection")
	}
}

// TestRejectTraversal verifies path traversal entries are rejected with an unsafe path error.
func TestRejectTraversal(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		"repo-main/../../etc/passwd": []byte(testContentNope),
	})
	dest := t.TempDir()

	result, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected traversal rejection")
	}

	if !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRejectUnsupportedEntryType verifies unsupported tar entry types are rejected.
func TestRejectUnsupportedEntryType(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithHeaders(t, []*tar.Header{
		{Name: "repo-main/device", Mode: consts.FilePerm644, Typeflag: tar.TypeChar},
	})

	result, err := archive.ExtractTarGz(bytes.NewReader(data), t.TempDir())
	iox.Discard(result)

	if err == nil {
		t.Fatal("expected unsupported entry type error")
	}
}

// TestSkipPAXGlobalHeader verifies PAX global header entries are skipped during extraction.
func TestSkipPAXGlobalHeader(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/github-pax.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	dest := extractToTemp(t, data)

	result, err := os.Stat(filepath.Join(dest, testTaskfileGoPath))
	iox.Discard(result)

	if err != nil {
		t.Fatal(err)
	}
}

func buildTarGz(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()

	builder := newTarGzBuilder(t)

	for name := range entries {
		builder.writeRegular(name, entries[name], consts.FilePerm644)
	}

	return builder.finish()
}

func buildTarGzWithHeaders(t *testing.T, headers []*tar.Header) []byte {
	t.Helper()

	builder := newTarGzBuilder(t)

	for i := range headers {
		builder.writeHeader(headers[i])
	}

	return builder.finish()
}

func buildTarGzWithMode(t *testing.T, spec *tarEntrySpec) []byte {
	t.Helper()

	builder := newTarGzBuilder(t)
	builder.writeRegular(spec.name, spec.content, spec.mode)

	return builder.finish()
}

func extractToTemp(t *testing.T, data []byte) string {
	t.Helper()

	dest := t.TempDir()

	result, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatal(err)
	}

	iox.Discard(result)

	return dest
}

func newTarGzBuilder(t *testing.T) *tarGzBuilder {
	t.Helper()

	builder := &tarGzBuilder{
		t:          t,
		gzipWriter: nil,
		tarWriter:  nil,
		buf:        bytes.Buffer{},
	}

	builder.gzipWriter = gzip.NewWriter(&builder.buf)
	builder.tarWriter = tar.NewWriter(builder.gzipWriter)

	return builder
}

func regularTarHeader(name string, size int64) *tar.Header {
	return &tar.Header{
		Name:       name,
		Mode:       consts.FilePerm644,
		Size:       size,
		Typeflag:   tar.TypeReg,
		Linkname:   consts.Empty,
		Uid:        consts.IndexZero,
		Gid:        consts.IndexZero,
		Uname:      consts.Empty,
		Gname:      consts.Empty,
		ModTime:    time.Time{},
		AccessTime: time.Time{},
		ChangeTime: time.Time{},
		Devmajor:   consts.IndexZero,
		Devminor:   consts.IndexZero,
		Xattrs:     nil,
		PAXRecords: nil,
		Format:     tar.FormatUnknown,
	}
}

func extractedFilePerm(t *testing.T, dest, rel string) os.FileMode {
	t.Helper()

	info, err := os.Stat(filepath.Join(dest, rel))
	if err != nil {
		t.Fatal(err)
	}

	return info.Mode().Perm()
}

func extractedIsDir(t *testing.T, dest, rel string) bool {
	t.Helper()

	info, err := os.Stat(filepath.Join(dest, rel))
	if err != nil {
		t.Fatal(err)
	}

	return info.IsDir()
}

func symlinkTarHeader(name, linkname string) *tar.Header {
	return &tar.Header{
		Name:       name,
		Mode:       consts.IndexZero,
		Size:       consts.IndexZero,
		Typeflag:   tar.TypeSymlink,
		Linkname:   linkname,
		Uid:        consts.IndexZero,
		Gid:        consts.IndexZero,
		Uname:      consts.Empty,
		Gname:      consts.Empty,
		ModTime:    time.Time{},
		AccessTime: time.Time{},
		ChangeTime: time.Time{},
		Devmajor:   consts.IndexZero,
		Devminor:   consts.IndexZero,
		Xattrs:     nil,
		PAXRecords: nil,
		Format:     tar.FormatUnknown,
	}
}

func (builder *tarGzBuilder) finish() []byte {
	builder.t.Helper()

	err := builder.tarWriter.Close()
	if err != nil {
		builder.t.Fatal(err)
	}

	err = builder.gzipWriter.Close()
	if err != nil {
		builder.t.Fatal(err)
	}

	return builder.buf.Bytes()
}

func (builder *tarGzBuilder) writeHeader(header *tar.Header) {
	builder.t.Helper()

	err := builder.tarWriter.WriteHeader(header)
	if err != nil {
		builder.t.Fatal(err)
	}

	if header.Typeflag != tar.TypeReg || header.Size <= consts.IndexZero {
		return
	}

	builder.writeRegularPayload(header.Size)
}

func (builder *tarGzBuilder) writeRegular(name string, content []byte, mode int64) {
	builder.t.Helper()

	header := regularTarHeader(name, int64(len(content)))

	header.Mode = mode

	err := builder.tarWriter.WriteHeader(header)
	if err != nil {
		builder.t.Fatal(err)
	}

	written, err := builder.tarWriter.Write(content)
	iox.Discard(written)

	if err != nil {
		builder.t.Fatal(err)
	}
}

func (builder *tarGzBuilder) writeRegularPayload(size int64) {
	builder.t.Helper()

	chunk := bytes.Repeat([]byte(testContentX), 32*1024)
	remaining := size

	for remaining > consts.IndexZero {
		writeSize := min(remaining, int64(len(chunk)))

		written, err := builder.tarWriter.Write(chunk[:writeSize])
		iox.Discard(written)

		if err != nil {
			builder.t.Fatal(err)
		}

		remaining -= writeSize
	}
}
