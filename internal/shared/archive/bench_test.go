// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"testing"
)

const (
	benchmarkSmallSize   = 2
	benchmarkMediumSize  = 20
	benchmarkLargeSize   = 100
	benchmarkArchiveMode = 0o644
)

// BenchmarkExtractTarGz measures archive extraction.
func BenchmarkExtractTarGz(b *testing.B) {
	for _, size := range []int{benchmarkSmallSize, benchmarkMediumSize, benchmarkLargeSize} {
		b.Run(fmt.Sprintf("files_%d", size), func(b *testing.B) {
			runExtractTarGzBenchmark(b, size)
		})
	}
}

func runExtractTarGzBenchmark(b *testing.B, size int) {
	b.Helper()

	data := benchmarkArchive(b, size)
	dest := b.TempDir()
	b.ResetTimer()

	runExtractTarGzIterations(b, data, dest)
}

func runExtractTarGzIterations(b *testing.B, data []byte, dest string) {
	b.Helper()

	for range b.N {
		root, err := ExtractTarGz(bytes.NewReader(data), dest)
		benchmarkExtractResult(b, root, err)
		b.StopTimer()

		err = removeBenchmarkRoot(root)
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
	}
}

func benchmarkExtractResult(b *testing.B, root string, err error) {
	b.Helper()

	if err != nil || root == "" {
		b.Fatalf("extract archive: %v", err)
	}
}

func benchmarkArchive(b *testing.B, size int) []byte {
	b.Helper()

	var buf bytes.Buffer

	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	for i := range size {
		writeBenchmarkArchiveEntry(b, tarWriter, i)
	}

	closeBenchmarkArchive(b, tarWriter, gzipWriter)

	return buf.Bytes()
}

func closeBenchmarkArchive(b *testing.B, tarWriter *tar.Writer, gz *gzip.Writer) {
	b.Helper()

	err := tarWriter.Close()
	if err != nil {
		b.Fatal(err)
	}

	err = gz.Close()
	if err != nil {
		b.Fatal(err)
	}
}

func writeBenchmarkArchiveEntry(b *testing.B, tarWriter *tar.Writer, index int) {
	b.Helper()

	body := []byte("version: \"3\"\n")
	header := &tar.Header{
		Name: fmt.Sprintf("store/taskfiles/module-%d/Taskfile.yml", index),
		Mode: benchmarkArchiveMode,
		Size: int64(len(body)),
	}

	err := tarWriter.WriteHeader(header)
	if err != nil {
		b.Fatal(err)
	}

	var written int64

	written, err = io.Copy(tarWriter, bytes.NewReader(body))
	if err != nil {
		b.Fatal(err)
	}

	if written == 0 {
		b.Fatal("empty archive entry")
	}
}

func removeBenchmarkRoot(root string) error {
	err := os.RemoveAll(root)
	if err != nil {
		return fmt.Errorf("remove benchmark root: %w", err)
	}

	return nil
}
