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
)

func regularTarHeader(name string, size int64) *tar.Header {
	return &tar.Header{
		Name:       name,
		Mode:       0o644,
		Size:       size,
		Typeflag:   tar.TypeReg,
		Linkname:   "",
		Uid:        0,
		Gid:        0,
		Uname:      "",
		Gname:      "",
		ModTime:    time.Time{},
		AccessTime: time.Time{},
		ChangeTime: time.Time{},
		Devmajor:   0,
		Devminor:   0,
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

	for name, content := range entries {
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

func TestExtractStripsSetuidMode(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithMode(
		t,
		"repo-main/taskfiles/go/Taskfile.yml",
		[]byte("version: \"3\"\n"),
		0o4755,
	)
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dest, "taskfiles/go/Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Fatalf("file mode = %o, want %o", info.Mode().Perm(), 0o755)
	}
}

func TestRejectNegativeFileMode(t *testing.T) {
	t.Parallel()

	data := buildTarGzWithMode(t, "repo-main/taskfiles/go/Taskfile.yml", []byte("x"), -1)
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err == nil {
		t.Fatal("expected negative file mode rejection")
	}
}

func TestExtractValidArchive(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		"repo-main/taskfiles/go/Taskfile.yml": []byte("version: \"3\"\n"),
	})
	dest := t.TempDir()

	_, err := archive.ExtractTarGz(bytes.NewReader(data), dest)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(dest, "taskfiles/go/Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRejectTraversal(t *testing.T) {
	t.Parallel()

	data := buildTarGz(t, map[string][]byte{
		"repo-main/../../etc/passwd": []byte("nope"),
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

func TestRejectSymlink(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	header := &tar.Header{
		Name:       "repo-main/link",
		Mode:       0,
		Size:       0,
		Typeflag:   tar.TypeSymlink,
		Linkname:   "/etc/passwd",
		Uid:        0,
		Gid:        0,
		Uname:      "",
		Gname:      "",
		ModTime:    time.Time{},
		AccessTime: time.Time{},
		ChangeTime: time.Time{},
		Devmajor:   0,
		Devminor:   0,
		Xattrs:     nil,
		PAXRecords: nil,
		Format:     tar.FormatUnknown,
	}

	err := tarWriter.WriteHeader(header)
	if err != nil {
		t.Fatal(err)
	}

	_ = tarWriter.Close()
	_ = gzipWriter.Close()
	dest := t.TempDir()

	_, err = archive.ExtractTarGz(bytes.NewReader(buf.Bytes()), dest)
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
}

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

	_, err = os.Stat(filepath.Join(dest, "taskfiles/go/Taskfile.yml"))
	if err != nil {
		t.Fatal(err)
	}
}
