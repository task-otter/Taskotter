// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"strconv"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/archive"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	benchArchiveModules = 40
	benchArchiveRoot    = "store-main/"
	benchModulePrefix   = "taskfiles/mod"
	benchReadmeBody     = "# module\n"
	benchTaskfileBody   = "version: \"3\"\nvars:\n  TOOL_VERSION: \"1.0.0\"\n" +
		"tasks:\n  lint:\n    cmds:\n      - echo lint\n"
)

func benchTarEntry(writer *tar.Writer, name, body string) error {
	header := &tar.Header{
		Name: name,
		Mode: consts.FilePerm644,
		Size: int64(len(body)),
	}

	err := writer.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("write tar header %q: %w", name, err)
	}

	err = iox.WriteStringFull(writer, body)
	if err != nil {
		return fmt.Errorf("write tar body %q: %w", name, err)
	}

	return nil
}

func benchModuleEntries(writer *tar.Writer) error {
	for idx := range benchArchiveModules {
		dir := benchArchiveRoot + benchModulePrefix + strconv.Itoa(idx)

		err := benchTarEntry(writer, dir+consts.TaskfileSuffix, benchTaskfileBody)
		if err != nil {
			return fmt.Errorf("write module taskfile: %w", err)
		}

		err = benchTarEntry(writer, dir+consts.PathSepString+consts.ReadmeMD, benchReadmeBody)
		if err != nil {
			return fmt.Errorf("write module readme: %w", err)
		}
	}

	return nil
}

func benchFillTar(tarWriter *tar.Writer) error {
	err := benchModuleEntries(tarWriter)
	if err != nil {
		return fmt.Errorf("write module entries: %w", err)
	}

	err = tarWriter.Close()
	if err != nil {
		return fmt.Errorf("close tar writer: %w", err)
	}

	return nil
}

func benchWriteArchive(buf *bytes.Buffer) error {
	gzipWriter := gzip.NewWriter(buf)

	err := benchFillTar(tar.NewWriter(gzipWriter))
	if err != nil {
		return fmt.Errorf("fill tar archive: %w", err)
	}

	err = gzipWriter.Close()
	if err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}

	return nil
}

// benchArchiveBytes builds a gzip-compressed tar archive shaped like a store download.
func benchArchiveBytes(b *testing.B) []byte {
	b.Helper()

	var buf bytes.Buffer

	err := benchWriteArchive(&buf)
	if err != nil {
		b.Fatal(err)
	}

	return buf.Bytes()
}

// BenchmarkExtractTarGz measures safe extraction of a store archive to disk.
func BenchmarkExtractTarGz(b *testing.B) {
	data := benchArchiveBytes(b)
	destDir := b.TempDir()

	for b.Loop() {
		root, err := archive.ExtractTarGz(bytes.NewReader(data), destDir)
		if err != nil {
			b.Fatal(err)
		}

		iox.Discard(root)
	}
}
