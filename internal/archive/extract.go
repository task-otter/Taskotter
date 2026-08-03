// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package archive provides safe extraction of gzip-compressed tar archives.
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

	"github.com/task-otter/Taskotter/internal/consts"
)

type (
	// ExtractError reports a safe-extraction failure.
	ExtractError struct {
		Message string
	}

	tarExtractor struct {
		reader     *tar.Reader
		destDir    string
		rootPrefix string
		total      int64
		rootSet    bool
	}
)

const (
	// MaxArchiveBytes is the maximum compressed archive size accepted for extraction.
	MaxArchiveBytes int64 = 100 * 1024 * 1024

	// MaxFileBytes is the maximum size of a single extracted file.
	MaxFileBytes int64 = 50 * 1024 * 1024

	// MaxTotalExtracted is the maximum total extracted payload size.
	MaxTotalExtracted int64 = 500 * 1024 * 1024

	dirPerm = 0o750

	maxTarFileMode int64 = 0o777
)

// Error implements the error interface, returning the extraction failure message.
func (e *ExtractError) Error() string {
	return e.Message
}

// ExtractTarGz extracts a gzip-compressed tar archive into destDir.
func ExtractTarGz(reader io.Reader, destDir string) (string, error) {
	err := os.MkdirAll(destDir, dirPerm)
	if err != nil {
		return consts.Empty, fmt.Errorf("create destination directory: %w", err)
	}

	root, err := extractTarGzStream(reader, destDir)
	if err != nil {
		return consts.Empty, fmt.Errorf("extract tar.gz stream: %w", err)
	}

	return root, nil
}

func extractTarGzStream(reader io.Reader, destDir string) (string, error) {
	gzipReader, err := gzip.NewReader(io.LimitReader(reader, MaxArchiveBytes+1))
	if err != nil {
		return consts.Empty, &ExtractError{Message: fmt.Sprintf("invalid gzip archive: %v", err)}
	}

	defer func() {
		closeErr := gzipReader.Close()

		if closeErr != nil && err == nil {
			err = fmt.Errorf("close gzip reader: %w", closeErr)
		}
	}()

	extractor := newTarExtractor(destDir, gzipReader)

	if err = extractor.run(); err != nil {
		return consts.Empty, fmt.Errorf("run extractor: %w", err)
	}

	return destDir, nil
}

func newTarExtractor(destDir string, gzipReader *gzip.Reader) *tarExtractor {
	return &tarExtractor{
		destDir:    destDir,
		reader:     tar.NewReader(gzipReader),
		total:      consts.IndexZero,
		rootPrefix: consts.Empty,
		rootSet:    false,
	}
}

func (extractor *tarExtractor) resolveRelativePath(name string) (string, bool) {
	parts := strings.Split(strings.Trim(name, consts.PathSepString), consts.PathSepString)

	if len(parts) == consts.IndexZero {
		return consts.Empty, false
	}

	if !extractor.rootSet {
		extractor.rootPrefix = parts[consts.IndexZero]
		extractor.rootSet = true
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(name, extractor.rootPrefix), consts.PathSepString)

	return rel, true
}

func (extractor *tarExtractor) skipMetadata(header *tar.Header) (bool, error) {
	err := discardTarEntry(extractor.reader, header.Size)
	if err != nil {
		return false, &ExtractError{
			Message: fmt.Sprintf("skip tar metadata entry %q: %v", header.Name, err),
		}
	}

	return true, nil
}

func (extractor *tarExtractor) shouldSkipEntry(header *tar.Header) (bool, error) {
	if isTarMetadataEntry(header.Typeflag) {
		skip, err := extractor.skipMetadata(header)
		if err != nil {
			return false, fmt.Errorf("skip metadata: %w", err)
		}

		return skip, nil
	}

	if !validTarPath(header.Name) {
		return false, &ExtractError{Message: fmt.Sprintf("unsafe tar path %q", header.Name)}
	}

	return false, nil
}

func (extractor *tarExtractor) writeDirEntry(target string) error {
	err := os.MkdirAll(target, dirPerm)
	if err != nil {
		return fmt.Errorf("create directory %q: %w", target, err)
	}

	return nil
}

func (extractor *tarExtractor) validateSize(size int64, name string) error {
	if size > MaxFileBytes {
		return &ExtractError{Message: fmt.Sprintf("file %q exceeds size limit", name)}
	}

	extractor.total += size

	if extractor.total > MaxTotalExtracted {
		return &ExtractError{Message: "total extracted size exceeds limit"}
	}

	return nil
}

func (extractor *tarExtractor) copyWithLimits(
	dst io.Writer,
	size int64,
	path string,
) (int64, error) {
	limited := io.LimitReader(extractor.reader, MaxFileBytes+1)

	written, err := io.Copy(dst, limited)
	if err != nil {
		return written, fmt.Errorf("write file %q: %w", path, err)
	}

	if written > MaxFileBytes {
		return written, &ExtractError{
			Message: fmt.Sprintf("file %q exceeds size limit during copy", path),
		}
	}

	if size >= consts.IndexZero && written != size {
		return written, &ExtractError{Message: fmt.Sprintf("short read for %q", path)}
	}

	return written, nil
}

func (extractor *tarExtractor) copyAndClose(file *os.File, size int64, path string) error {
	written, err := extractor.copyWithLimits(file, size, path)

	_ = written

	if err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			// Ignore closing error on copy failure.
		}

		return fmt.Errorf("copy with limits: %w", err)
	}

	closeErr := file.Close()
	if closeErr != nil {
		return fmt.Errorf("close file %q: %w", path, closeErr)
	}

	return nil
}

func (extractor *tarExtractor) writeRegularFile(path string, mode, size int64) error {
	cleanPath := filepath.Clean(path)

	file, err := openTarget(cleanPath, mode)
	if err != nil {
		return fmt.Errorf("open target: %w", err)
	}

	err = extractor.copyAndClose(file, size, cleanPath)
	if err != nil {
		return fmt.Errorf("copy and close: %w", err)
	}

	return nil
}

func (extractor *tarExtractor) writeRegEntry(header *tar.Header, target string) error {
	err := extractor.validateSize(header.Size, header.Name)
	if err != nil {
		return fmt.Errorf("validate size: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(target), dirPerm)
	if err != nil {
		return fmt.Errorf("create parent directory for %q: %w", target, err)
	}

	err = extractor.writeRegularFile(target, header.Mode, header.Size)
	if err != nil {
		return fmt.Errorf("write regular file: %w", err)
	}

	return nil
}

func (extractor *tarExtractor) writeEntry(header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		err := extractor.writeDirEntry(target)
		if err != nil {
			return fmt.Errorf("write dir entry: %w", err)
		}

		return nil
	case tar.TypeReg:
		err := extractor.writeRegEntry(header, target)
		if err != nil {
			return fmt.Errorf("write reg entry: %w", err)
		}

		return nil
	case tar.TypeSymlink, tar.TypeLink:
		return &ExtractError{Message: fmt.Sprintf("unsupported link entry %q", header.Name)}
	default:
		return &ExtractError{Message: fmt.Sprintf("unsupported tar entry type for %q", header.Name)}
	}
}

func (extractor *tarExtractor) writeResolvedEntry(header *tar.Header, rel string) error {
	target := filepath.Join(extractor.destDir, rel)

	err := ensureInside(extractor.destDir, target)
	if err != nil {
		return fmt.Errorf("ensure inside: %w", err)
	}

	err = extractor.writeEntry(header, target)
	if err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	return nil
}

func (extractor *tarExtractor) processHeader(header *tar.Header) error {
	skip, err := extractor.shouldSkipEntry(header)
	if err != nil {
		return fmt.Errorf("should skip entry: %w", err)
	}

	if skip {
		return nil
	}

	name := filepath.Clean(header.Name)
	rel, ok := extractor.resolveRelativePath(name)

	if !ok || isSkippableRootDir(rel, header.Typeflag) {
		return nil
	}

	if err = extractor.writeResolvedEntry(header, rel); err != nil {
		return fmt.Errorf("write resolved entry: %w", err)
	}

	return nil
}

func (extractor *tarExtractor) run() error {
	for {
		header, err := extractor.reader.Next()

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return &ExtractError{Message: fmt.Sprintf("read tar entry: %v", err)}
		}

		if err = extractor.processHeader(header); err != nil {
			return fmt.Errorf("process header: %w", err)
		}
	}
}

func isSkippableRootDir(rel string, typeflag byte) bool {
	return (rel == consts.Empty || rel == ".") && typeflag == tar.TypeDir
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
	if size <= consts.IndexZero {
		return nil
	}

	n, err := io.CopyN(io.Discard, reader, size)

	_ = n

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

	if strings.HasPrefix(clean, consts.PathParent) {
		return false
	}

	return !strings.Contains(name, "\\")
}

func ensureInside(base, target string) error {
	absBase, absTarget, err := absPaths(base, target)
	if err != nil {
		return fmt.Errorf("abs paths: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return &ExtractError{Message: fmt.Sprintf("path escapes extraction directory: %v", err)}
	}

	if isPathEscaping(rel) {
		return &ExtractError{Message: "path escapes extraction directory"}
	}

	return nil
}

func absPaths(base, target string) (absBase, absTarget string, err error) {
	absBase, err = filepath.Abs(base)
	if err != nil {
		return consts.Empty, consts.Empty, fmt.Errorf("resolve base path: %w", err)
	}

	absTarget, err = filepath.Abs(target)
	if err != nil {
		return consts.Empty, consts.Empty, fmt.Errorf("resolve target path: %w", err)
	}

	return absBase, absTarget, nil
}

func isPathEscaping(rel string) bool {
	return rel == consts.PathParent ||
		strings.HasPrefix(rel, consts.PathParent+string(os.PathSeparator))
}

func safeTarFileMode(mode int64) (os.FileMode, error) {
	if mode < consts.IndexZero {
		return consts.IndexZero, &ExtractError{Message: fmt.Sprintf("invalid file mode %o", mode)}
	}

	perm := mode & maxTarFileMode

	if perm > maxTarFileMode {
		return consts.IndexZero, &ExtractError{
			Message: fmt.Sprintf("file mode %o out of range", mode),
		}
	}

	return os.FileMode(perm), nil
}

func openTarget(path string, mode int64) (*os.File, error) {
	perm, err := safeTarFileMode(mode)
	if err != nil {
		return nil, fmt.Errorf("safe tar file mode: %w", err)
	}

	file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", path, err)
	}

	return file, nil
}
