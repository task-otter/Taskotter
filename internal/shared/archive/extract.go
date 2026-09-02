// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package archive provides safe extraction of gzip-compressed tar archives.
//
//nolint:funcorder // extractor methods follow tar walk order
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

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
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

	errFmtWriteDirEntry = "write dir entry: %w"

	errFmtWriteEntry = "write entry: %w"

	errFmtEnsureValidTarPath = "ensure valid tar path: %w"
)

//nolint:gochecknoglobals // seam so tests can reach the filepath.Abs failure branches
var absPath = filepath.Abs

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

func absPaths(base, target string) (absBase, absTarget string, err error) {
	absBase, err = absPath(base)
	if err != nil {
		return consts.Empty, consts.Empty, fmt.Errorf("resolve base path: %w", err)
	}

	absTarget, err = absPath(target)
	if err != nil {
		return consts.Empty, consts.Empty, fmt.Errorf("resolve target path: %w", err)
	}

	return absBase, absTarget, nil
}

func closeGzipReader(gzipReader io.Closer) error {
	closeErr := gzipReader.Close()
	if closeErr != nil {
		return fmt.Errorf("close gzip reader: %w", closeErr)
	}

	return nil
}

func discardTarEntry(reader io.Reader, size int64) error {
	if size <= consts.IndexZero {
		return nil
	}

	n, err := io.CopyN(io.Discard, reader, size)

	iox.Discard(n)

	if err != nil {
		return fmt.Errorf("discard tar entry: %w", err)
	}

	return nil
}

func ensureInside(base, target string) error {
	absBase, absTarget, err := absPaths(base, target)
	if err != nil {
		return fmt.Errorf("abs paths: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)

	if err != nil || isPathEscaping(rel) {
		return &ExtractError{Message: "path escapes extraction directory"}
	}

	return nil
}

func extractTarGzStream(reader io.Reader, destDir string) (string, error) {
	gzipReader, err := openGzipReader(reader)
	if err != nil {
		return consts.Empty, fmt.Errorf("open gzip reader: %w", err)
	}

	root, err := extractFromGzipReader(gzipReader, destDir)
	if err != nil {
		return consts.Empty, fmt.Errorf("extract from gzip reader: %w", err)
	}

	return root, nil
}

func extractFromGzipReader(gzipReader *gzip.Reader, destDir string) (root string, err error) {
	defer func() {
		err = preferErr(err, closeGzipReader(gzipReader))
	}()

	root, err = runTarExtractor(destDir, gzipReader)
	if err != nil {
		return consts.Empty, fmt.Errorf("run tar extractor: %w", err)
	}

	return root, nil
}

// preferErr keeps err when set, otherwise reports the cleanup error.
func preferErr(err, closeErr error) error {
	if err != nil {
		return err
	}

	return closeErr
}

func isPathEscaping(rel string) bool {
	return rel == consts.PathParent ||
		strings.HasPrefix(rel, consts.PathParent+string(os.PathSeparator))
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

func linkEntryError(name string) error {
	return &ExtractError{Message: fmt.Sprintf("unsupported link entry %q", name)}
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

func openGzipReader(reader io.Reader) (*gzip.Reader, error) {
	gzipReader, err := gzip.NewReader(io.LimitReader(reader, MaxArchiveBytes+1))
	if err != nil {
		return nil, &ExtractError{Message: fmt.Sprintf("invalid gzip archive: %v", err)}
	}

	return gzipReader, nil
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

func runTarExtractor(destDir string, gzipReader *gzip.Reader) (string, error) {
	extractor := newTarExtractor(destDir, gzipReader)

	err := extractor.run()
	if err != nil {
		return consts.Empty, fmt.Errorf("run extractor: %w", err)
	}

	return destDir, nil
}

func safeTarFileMode(mode int64) (os.FileMode, error) {
	if mode < consts.IndexZero {
		return consts.IndexZero, &ExtractError{Message: fmt.Sprintf("invalid file mode %o", mode)}
	}

	// Masking with maxTarFileMode keeps only the permission bits, dropping setuid,
	// setgid, and sticky bits from the archive.
	return os.FileMode(mode & maxTarFileMode), nil
}

func unsupportedEntryError(name string) error {
	return &ExtractError{Message: fmt.Sprintf("unsupported tar entry type for %q", name)}
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

func validateCopySize(written, size int64, path string) error {
	if written > MaxFileBytes {
		return &ExtractError{
			Message: fmt.Sprintf("file %q exceeds size limit during copy", path),
		}
	}

	if size >= consts.IndexZero && written != size {
		return &ExtractError{Message: fmt.Sprintf("short read for %q", path)}
	}

	return nil
}

func writeDirEntry(target string) error {
	err := os.MkdirAll(target, dirPerm)
	if err != nil {
		return fmt.Errorf("create directory %q: %w", target, err)
	}

	return nil
}

func writeDirEntryWithWrap(target string) error {
	err := writeDirEntry(target)
	if err != nil {
		return fmt.Errorf(errFmtWriteDirEntry, err)
	}

	return nil
}

// Error implements the error interface, returning the extraction failure message.
func (e *ExtractError) Error() string {
	return e.Message
}

func (extractor *tarExtractor) copyAndClose(file *os.File, size int64, path string) error {
	written, err := extractor.copyLim(file, size, path)

	iox.Discard(written)

	if err != nil {
		closeErr := file.Close()
		iox.Discard(closeErr)

		return fmt.Errorf("copy with limits: %w", err)
	}

	closeErr := file.Close()
	if closeErr != nil {
		return fmt.Errorf("close file %q: %w", path, closeErr)
	}

	return nil
}

func (extractor *tarExtractor) copyTo(dst io.Writer, path string) (int64, error) {
	limited := io.LimitReader(extractor.reader, MaxFileBytes+1)

	written, err := io.Copy(dst, limited)
	if err != nil {
		return written, fmt.Errorf("write file %q: %w", path, err)
	}

	return written, nil
}

func (extractor *tarExtractor) copyLim(dst io.Writer, size int64, path string) (int64, error) {
	written, err := extractor.copyTo(dst, path)
	if err != nil {
		return written, fmt.Errorf("copy to: %w", err)
	}

	err = validateCopySize(written, size, path)
	if err != nil {
		return written, fmt.Errorf("validate copy size: %w", err)
	}

	return written, nil
}

func (extractor *tarExtractor) nextHeader() (*tar.Header, error) {
	header, err := extractor.reader.Next()

	if errors.Is(err, io.EOF) {
		return nil, io.EOF
	}

	if err != nil {
		return nil, &ExtractError{Message: fmt.Sprintf("read tar entry: %v", err)}
	}

	return header, nil
}

func (extractor *tarExtractor) processContentEntry(header *tar.Header) error {
	name := filepath.Clean(header.Name)
	rel, ok := extractor.resolveRelativePath(name)

	if !ok || isSkippableRootDir(rel, header.Typeflag) {
		return nil
	}

	err := extractor.writeResolvedEntry(header, rel)
	if err != nil {
		return fmt.Errorf("write resolved entry: %w", err)
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

	err = extractor.processContentEntry(header)
	if err != nil {
		return fmt.Errorf("process content entry: %w", err)
	}

	return nil
}

func (extractor *tarExtractor) resolveRelativePath(name string) (string, bool) {
	// strings.Split always yields at least one element, so parts[0] is safe here.
	parts := strings.Split(strings.Trim(name, consts.PathSepString), consts.PathSepString)

	if !extractor.rootSet {
		extractor.rootPrefix = parts[consts.IndexZero]
		extractor.rootSet = true
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(name, extractor.rootPrefix), consts.PathSepString)

	return rel, true
}

func (extractor *tarExtractor) run() error {
	for {
		cont, err := extractor.step()

		if cont {
			continue
		}

		if err != nil {
			return fmt.Errorf("extractor step: %w", err)
		}

		return nil
	}
}

//nolint:nestif // metadata vs regular entries require distinct skip paths
func (extractor *tarExtractor) shouldSkipEntry(header *tar.Header) (bool, error) {
	if isTarMetadataEntry(header.Typeflag) {
		skip, err := extractor.skipMetadataWithWrap(header)
		if err != nil {
			return false, fmt.Errorf("skip metadata entry: %w", err)
		}

		return skip, nil
	}

	err := checkEntryPath(header.Name)
	if err != nil {
		return false, fmt.Errorf("validate entry path: %w", err)
	}

	return false, nil
}

func (extractor *tarExtractor) skipMetadataWithWrap(header *tar.Header) (bool, error) {
	skip, err := extractor.skipMetadata(header)
	if err != nil {
		return false, fmt.Errorf("skip metadata: %w", err)
	}

	return skip, nil
}

func checkEntryPath(name string) error {
	err := ensureValidTarPath(name)
	if err != nil {
		return fmt.Errorf(errFmtEnsureValidTarPath, err)
	}

	return nil
}

func ensureValidTarPath(name string) error {
	if validTarPath(name) {
		return nil
	}

	return &ExtractError{Message: fmt.Sprintf("unsafe tar path %q", name)}
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

func (extractor *tarExtractor) step() (continueLoop bool, err error) {
	header, err := extractor.nextHeader()

	if errors.Is(err, io.EOF) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("next header: %w", err)
	}

	err = extractor.processHeader(header)
	if err != nil {
		return false, fmt.Errorf("process header: %w", err)
	}

	return true, nil
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

func writeDirTarget(target string) error {
	err := writeDirEntryWithWrap(target)
	if err != nil {
		return fmt.Errorf("write dir entry: %w", err)
	}

	return nil
}

func (extractor *tarExtractor) writeEntry(header *tar.Header, target string) error {
	err := extractor.dispatchWriteEntry(header, target)
	if err != nil {
		return fmt.Errorf(errFmtWriteEntry, err)
	}

	return nil
}

//nolint:wrapcheck // writeEntry wraps errors from this dispatcher
func (extractor *tarExtractor) dispatchWriteEntry(header *tar.Header, target string) error {
	if header.Typeflag == tar.TypeDir {
		return writeDirTarget(target)
	}

	if header.Typeflag == tar.TypeReg {
		return extractor.writeRegTarget(header, target)
	}

	if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
		return linkEntryError(header.Name)
	}

	return unsupportedEntryError(header.Name)
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

func (extractor *tarExtractor) writeRegTarget(header *tar.Header, target string) error {
	err := extractor.writeRegEntry(header, target)
	if err != nil {
		return fmt.Errorf("write reg entry: %w", err)
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

func (extractor *tarExtractor) writeResolvedEntry(header *tar.Header, rel string) error {
	target := filepath.Join(extractor.destDir, rel)

	err := ensureInside(extractor.destDir, target)
	if err != nil {
		return fmt.Errorf("ensure inside: %w", err)
	}

	err = extractor.writeEntry(header, target)
	if err != nil {
		return fmt.Errorf(errFmtWriteEntry, err)
	}

	return nil
}
