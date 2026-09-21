// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/archive"
	"github.com/task-otter/Taskotter/internal/shared/consts"
)

func createBenchmarkTarGz(b *testing.B) []byte {
	b.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	headers := []*tar.Header{
		{Name: "repo-main/", Mode: consts.FilePerm755, Typeflag: tar.TypeDir},
		{Name: "repo-main/taskfiles/", Mode: consts.FilePerm755, Typeflag: tar.TypeDir},
		{Name: "repo-main/taskfiles/go/Taskfile.yml", Mode: consts.FilePerm644, Size: 100, Typeflag: tar.TypeReg},
		{Name: "repo-main/taskfiles/eslint/Taskfile.yml", Mode: consts.FilePerm644, Size: 200, Typeflag: tar.TypeReg},
	}
	content := bytes.Repeat([]byte("version: \"3\"\nvars:\n  FOO: bar\n"), 20)

	for _, h := range headers {
		if err := tw.WriteHeader(h); err != nil {
			b.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write(content[:h.Size]); err != nil {
				b.Fatal(err)
			}
		}
	}
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

func BenchmarkExtractTarGz(b *testing.B) {
	data := createBenchmarkTarGz(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		targetDir := b.TempDir()
		_, err := archive.ExtractTarGz(bytes.NewReader(data), targetDir)
		if err != nil {
			b.Fatal(err)
		}
		_ = os.RemoveAll(targetDir)
	}
}
