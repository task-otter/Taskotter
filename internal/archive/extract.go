// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxArchiveBytes is the maximum compressed archive size accepted for extraction.
	MaxArchiveBytes int64 = 100 * 1024 * 1024
	// MaxFileBytes is the maximum size of a single extracted file.
	MaxFileBytes int64 = 50 * 1024 * 1024
	// MaxTotalExtracted is the maximum total extracted payload size.
	MaxTotalExtracted int64 = 500 * 1024 * 1024
)

const dirPerm = 0o750

// ExtractError reports a safe-extraction failure.
type ExtractError struct {
	Message string
}

func (e *ExtractError) Error() string {
	return e.Message
}

// ExtractTarGz extracts a gzip-compressed tar archive into destDir.
func ExtractTarGz(reader io.Reader, destDir string) (string, error) {
	err := os.MkdirAll(destDir, dirPerm)
	if err != nil {
		return "", fmt.Errorf("create destination directory: %w", err)
	}

	gzipReader, err := gzip.NewReader(io.LimitReader(reader, MaxArchiveBytes+1))
	if err != nil {
		return "", &ExtractError{Message: fmt.Sprintf("invalid gzip archive: %v", err)}
	}

	defer func() { _ = gzipReader.Close() }()

	extractor := &tarExtractor{
		destDir:    destDir,
		reader:     tar.NewReader(gzipReader),
		total:      0,
		rootPrefix: "",
		rootSet:    false,
	}

	err = extractor.run()
	if err != nil {
		return "", err
	}

	return destDir, nil
}

type tarExtractor struct {
	reader     *tar.Reader
	destDir    string
	rootPrefix string
	total      int64
	rootSet    bool
}

func (e *tarExtractor) run() error {
	for {
		header, err := e.reader.Next()

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return &ExtractError{Message: fmt.Sprintf("read tar entry: %v", err)}
		}

		err = e.processHeader(header)
		if err != nil {
			return err
		}
	}
}

func (e *tarExtractor) processHeader(header *tar.Header) error {
	if isTarMetadataEntry(header.Typeflag) {
		err := discardTarEntry(e.reader, header.Size)
		if err != nil {
			return &ExtractError{
				Message: fmt.Sprintf("skip tar metadata entry %q: %v", header.Name, err),
			}
		}

		return nil
	}

	if !validTarPath(header.Name) {
		return &ExtractError{Message: fmt.Sprintf("unsafe tar path %q", header.Name)}
	}

	name := filepath.Clean(header.Name)

	parts := strings.Split(strings.Trim(name, "/"), "/")

	if len(parts) == 0 {
		return nil
	}

	if !e.rootSet {
		e.rootPrefix = parts[0]
		e.rootSet = true
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(name, e.rootPrefix), "/")

	if rel == "" || rel == "." {
		if header.Typeflag == tar.TypeDir {
			return nil
		}
	}

	target := filepath.Join(e.destDir, rel)

	err := ensureInside(e.destDir, target)
	if err != nil {
		return err
	}

	return e.writeEntry(header, target)
}

func (e *tarExtractor) writeEntry(header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		err := os.MkdirAll(target, dirPerm)
		if err != nil {
			return fmt.Errorf("create directory %q: %w", target, err)
		}
	case tar.TypeReg:
		if header.Size > MaxFileBytes {
			return &ExtractError{Message: fmt.Sprintf("file %q exceeds size limit", header.Name)}
		}

		e.total += header.Size

		if e.total > MaxTotalExtracted {
			return &ExtractError{Message: "total extracted size exceeds limit"}
		}

		err := os.MkdirAll(filepath.Dir(target), dirPerm)
		if err != nil {
			return fmt.Errorf("create parent directory for %q: %w", target, err)
		}

		err = writeRegularFile(target, e.reader, header.Size, header.Mode)
		if err != nil {
			return err
		}
	case tar.TypeSymlink, tar.TypeLink:
		return &ExtractError{Message: fmt.Sprintf("unsupported link entry %q", header.Name)}
	default:
		return &ExtractError{Message: fmt.Sprintf("unsupported tar entry type for %q", header.Name)}
	}

	return nil
}

func isTarMetadataEntry(typeflag byte) bool {
	switch typeflag {
	case tar.TypeXGlobalHeader, tar.TypeXHeader, tar.TypeGNULongName, tar.TypeGNULongLink:
		return true
	default:
		return false
	}
}

func discardTarEntry(reader io.Reader, size int64) error {
	if size <= 0 {
		return nil
	}

	_, err := io.CopyN(io.Discard, reader, size)
	if err != nil {
		return fmt.Errorf("discard tar entry: %w", err)
	}

	return nil
}

func validTarPath(name string) bool {
	if filepath.IsAbs(name) {
		return false
	}

	clean := filepath.Clean(name)

	if strings.HasPrefix(clean, "..") {
		return false
	}

	return !strings.Contains(name, "\\")
}

func ensureInside(base, target string) error {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve base path: %w", err)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return &ExtractError{Message: fmt.Sprintf("path escapes extraction directory: %v", err)}
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return &ExtractError{Message: "path escapes extraction directory"}
	}

	return nil
}

const maxTarFileMode int64 = 0o777

func safeTarFileMode(mode int64) (os.FileMode, error) {
	if mode < 0 {
		return 0, &ExtractError{Message: fmt.Sprintf("invalid file mode %o", mode)}
	}

	perm := mode & maxTarFileMode

	if perm > maxTarFileMode {
		return 0, &ExtractError{Message: fmt.Sprintf("file mode %o out of range", mode)}
	}

	return os.FileMode(perm), nil
}

func writeRegularFile(path string, reader io.Reader, size int64, mode int64) error {
	cleanPath := filepath.Clean(path)

	perm, err := safeTarFileMode(mode)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("open file %q: %w", cleanPath, err)
	}

	limited := io.LimitReader(reader, MaxFileBytes+1)

	written, err := io.Copy(file, limited)
	if err != nil {
		_ = file.Close()

		return fmt.Errorf("write file %q: %w", cleanPath, err)
	}

	if written > MaxFileBytes {
		_ = file.Close()

		return &ExtractError{
			Message: fmt.Sprintf("file %q exceeds size limit during copy", cleanPath),
		}
	}

	if size >= 0 && written != size {
		_ = file.Close()

		return &ExtractError{Message: fmt.Sprintf("short read for %q", cleanPath)}
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("close file %q: %w", cleanPath, err)
	}

	return nil
}
