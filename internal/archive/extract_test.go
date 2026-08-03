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

	"github.com/task-otter/Taskotter/internal/archive"
	"github.com/task-otter/Taskotter/internal/consts"
)

const (
	testRepoTaskfileGo = "repo-main/taskfiles/go/Taskfile.yml"
	testTaskfileGoPath = "taskfiles/go/Taskfile.yml"
	testContentX       = "x"
	testContentNope    = "nope"
)

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

func buildTarGz(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for name := range entries {
		content := entries[name]
		header := regularTarHeader(name, int64(len(content)))

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

func buildTarGzWithMode(t *testing.T, name string, content []byte, mode int64) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	header := regularTarHeader(name, int64(len(content)))

	header.Mode = mode

	err := tarWriter.WriteHeader(header)
	if err != nil {
		t.Fatal(err)
	}

	_, err = tarWriter.Write(content)
	if err != nil {
		t.Fatal(err)
	}

	err = tarWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = gzipWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func buildTarGzWithHeaders(t *testing.T, headers []*tar.Header) []byte {
	t.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for i := range headers {
		header := headers[i]

		err := tarWriter.WriteHeader(header)
		if err != nil {
			t.Fatal(err)
		}

		if header.Typeflag == tar.TypeReg && header.Size > consts.IndexZero {
			chunk := bytes.Repeat([]byte(testContentX), 32*1024)

			remaining := header.Size

			for remaining > consts.IndexZero {
				writeSize := min(remaining, int64(len(chunk)))

				_, err := tarWriter.Write(chunk[:writeSize])
				if err != nil {
					t.Fatal(err)
				}

				remaining -= writeSize
			}
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

// TestExtractStripsSetuidMode verifies the setuid bit is stripped from extracted file modes.
func TestExtractStripsSetuidMode(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithMode(
		t,
		testRepoTaskfileGo,
		[]byte("version: \"3\"\n"),
		consts.FilePerm4755,
	)
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dest, testTaskfileGoPath))
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != consts.FilePerm755 {
		t.Fatalf("file mode = %o, want %o", info.Mode().Perm(), consts.FilePerm755)
	}
}

// TestRejectNegativeFileMode verifies a negative tar file mode is rejected.
func TestRejectNegativeFileMode(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithMode(t, testRepoTaskfileGo, []byte(testContentX), -1)
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err == nil {
		t.Fatal("expected negative file mode rejection")
	}
}

// TestExtractValidArchive verifies a well-formed archive extracts its files successfully.
func TestExtractValidArchive(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		testRepoTaskfileGo: []byte("version: \"3\"\n"),
	})
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(dest, testTaskfileGoPath))
	if err != nil {
		t.Fatal(err)
	}
}

// TestRejectInvalidGzip verifies non-gzip input is rejected with an error.
func TestRejectInvalidGzip(t *testing.T) {
	t.Parallel()

	_, err := archive.ExtractTarGz(strings.NewReader("not gzip"), t.TempDir())
	if err == nil {
		t.Fatal("expected invalid gzip error")
	}
}

// TestExtractDirectoryEntry verifies directory entries are created on disk.
func TestExtractDirectoryEntry(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithHeaders(t, []*tar.Header{
		{Name: "repo-main/", Mode: consts.FilePerm755, Typeflag: tar.TypeDir},
		{Name: "repo-main/taskfiles", Mode: consts.FilePerm755, Typeflag: tar.TypeDir},
		regularTarHeader(testRepoTaskfileGo, int64(len("version: \"3\"\n"))),
	})
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dest, "taskfiles"))
	if err != nil {
		t.Fatal(err)
	}

	if !info.IsDir() {
		t.Fatal("taskfiles should be a directory")
	}
}

// TestRejectBackslashPath verifies entry names containing backslashes are rejected.
func TestRejectBackslashPath(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		`repo-main\taskfiles\go\Taskfile.yml`: []byte(testContentNope),
	})
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err == nil {
		t.Fatal("expected backslash path rejection")
	}
}

// TestRejectUnsupportedEntryType verifies unsupported tar entry types are rejected.
func TestRejectUnsupportedEntryType(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithHeaders(t, []*tar.Header{
		{Name: "repo-main/device", Mode: consts.FilePerm644, Typeflag: tar.TypeChar},
	})

	_, err := archive.ExtractTarGz(bytes.NewReader(data), t.TempDir())
	if err == nil {
		t.Fatal("expected unsupported entry type error")
	}
}

// TestRejectFileLargerThanLimit verifies files exceeding the size limit are rejected.
func TestRejectFileLargerThanLimit(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithHeaders(t, []*tar.Header{
		regularTarHeader("repo-main/huge.bin", archive.MaxFileBytes+1),
	})

	_, err := archive.ExtractTarGz(bytes.NewReader(data), t.TempDir())
	if err == nil {
		t.Fatal("expected file size limit error")
	}
}

// TestRejectTraversal verifies path traversal entries are rejected with an unsafe path error.
func TestRejectTraversal(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		"repo-main/../../etc/passwd": []byte(testContentNope),
	})
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err == nil {
		t.Fatal("expected traversal rejection")
	}

	if !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestRejectSymlink verifies symlink entries are rejected during extraction.
func TestRejectSymlink(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	header := &tar.Header{
		Name:       "repo-main/link",
		Mode:       consts.IndexZero,
		Size:       consts.IndexZero,
		Typeflag:   tar.TypeSymlink,
		Linkname:   "/etc/passwd",
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

	err := tarWriter.WriteHeader(header)
	if err != nil {
		t.Fatal(err)
	}

	err = tarWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	err = gzipWriter.Close()
	if err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()

	_, err = archive.ExtractTarGz(bytes.NewReader(buf.Bytes()), dest)
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
}

// TestSkipPAXGlobalHeader verifies PAX global header entries are skipped during extraction.
func TestSkipPAXGlobalHeader(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/github-pax.tar.gz")
	if err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()

	_, err = archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(dest, testTaskfileGoPath))
	if err != nil {
		t.Fatal(err)
	}
}
