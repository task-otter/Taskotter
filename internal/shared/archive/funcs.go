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

	"github.com/task-otter/Taskotter/internal/shared/archive/safety"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// ExtractTarGz extracts a gzip-compressed tar archive into destDir.
func ExtractTarGz(reader Reader, destDir string) (string, error) {
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
	return absPathsWith(filepath.Abs, base, target)
}

func absPathsWith(
	resolve func(string) (string, error),
	base, target string,
) (absBase, absTarget string, err error) {
	absBase, err = resolve(base)
	if err != nil {
		return consts.Empty, consts.Empty, fmt.Errorf("resolve base path: %w", err)
	}

	absTarget, err = resolve(target)
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
	return ensureInsideWith(filepath.Abs, base, target)
}

func ensureInsideWith(resolve func(string) (string, error), base, target string) error {
	absBase, absTarget, err := absPathsWith(resolve, base, target)
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
	return safety.Escapes(rel)
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

	err := executeTarExtractor(extractor)
	if err != nil {
		return consts.Empty, fmt.Errorf("run extractor: %w", err)
	}

	return destDir, nil
}

func safeTarFileMode(mode int64) (os.FileMode, error) {
	if mode < consts.IndexZero {
		return consts.IndexZero, &ExtractError{Message: fmt.Sprintf("invalid file mode %o", mode)}
	}

	return os.FileMode(mode & maxTarFileMode), nil
}

func unsupportedEntryError(name string) error {
	return &ExtractError{Message: fmt.Sprintf("unsupported tar entry type for %q", name)}
}

func validTarPath(name string) bool {
	return safety.IsSafeTarPath(name)
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

func copyAndClose(extractor *tarExtractor, args *copyCloseArgs) error {
	written, err := copyLim(
		extractor,
		&copyLimArgs{dst: args.file, size: args.size, path: args.path},
	)

	iox.Discard(written)

	if err != nil {
		closeErr := args.file.Close()
		iox.Discard(closeErr)

		return fmt.Errorf("copy with limits: %w", err)
	}

	closeErr := args.file.Close()
	if closeErr != nil {
		return fmt.Errorf("close file %q: %w", args.path, closeErr)
	}

	return nil
}

func copyTo(extractor *tarExtractor, dst io.Writer, path string) (int64, error) {
	limited := io.LimitReader(extractor.reader, MaxFileBytes+1)

	written, err := io.Copy(dst, limited)
	if err != nil {
		return written, fmt.Errorf("write file %q: %w", path, err)
	}

	return written, nil
}

func copyLim(extractor *tarExtractor, args *copyLimArgs) (int64, error) {
	written, err := copyTo(extractor, args.dst, args.path)
	if err != nil {
		return written, fmt.Errorf("copy to: %w", err)
	}

	err = validateCopySize(written, args.size, args.path)
	if err != nil {
		return written, fmt.Errorf("validate copy size: %w", err)
	}

	return written, nil
}

func nextHeader(extractor *tarExtractor) (*tar.Header, error) {
	header, err := extractor.reader.Next()

	if errors.Is(err, io.EOF) {
		return nil, io.EOF
	}

	if err != nil {
		return nil, &ExtractError{Message: fmt.Sprintf("read tar entry: %v", err)}
	}

	return header, nil
}

func processContentEntry(extractor *tarExtractor, header *tar.Header) error {
	name := filepath.Clean(header.Name)
	rel, ok := resolveRelativePath(extractor, name)

	if !ok || isSkippableRootDir(rel, header.Typeflag) {
		return nil
	}

	err := writeResolvedEntry(extractor, header, rel)
	if err != nil {
		return fmt.Errorf("write resolved entry: %w", err)
	}

	return nil
}

func processHeader(extractor *tarExtractor, header *tar.Header) error {
	skip, err := shouldSkipEntry(extractor, header)
	if err != nil {
		return fmt.Errorf("should skip entry: %w", err)
	}

	if skip {
		return nil
	}

	err = processContentEntry(extractor, header)
	if err != nil {
		return fmt.Errorf("process content entry: %w", err)
	}

	return nil
}

func resolveRelativePath(extractor *tarExtractor, name string) (string, bool) {
	parts := strings.Split(strings.Trim(name, consts.PathSepString), consts.PathSepString)

	if !extractor.rootSet {
		extractor.rootPrefix = parts[consts.IndexZero]
		extractor.rootSet = true
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(name, extractor.rootPrefix), consts.PathSepString)

	return rel, true
}

func executeTarExtractor(extractor *tarExtractor) error {
	for {
		cont, err := step(extractor)

		if cont {
			continue
		}

		if err != nil {
			return fmt.Errorf("extractor step: %w", err)
		}

		return nil
	}
}

func shouldSkipEntry(extractor *tarExtractor, header *tar.Header) (bool, error) {
	if isTarMetadataEntry(header.Typeflag) {
		skip, err := skipMetadataWithWrap(extractor, header)
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

func skipMetadataWithWrap(extractor *tarExtractor, header *tar.Header) (bool, error) {
	skip, err := skipMetadata(extractor, header)
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

func skipMetadata(extractor *tarExtractor, header *tar.Header) (bool, error) {
	err := discardTarEntry(extractor.reader, header.Size)
	if err != nil {
		return false, &ExtractError{
			Message: fmt.Sprintf("skip tar metadata entry %q: %v", header.Name, err),
		}
	}

	return true, nil
}

func step(extractor *tarExtractor) (continueLoop bool, err error) {
	header, err := nextHeader(extractor)

	if errors.Is(err, io.EOF) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("next header: %w", err)
	}

	err = processHeader(extractor, header)
	if err != nil {
		return false, fmt.Errorf("process header: %w", err)
	}

	return true, nil
}

func validateSize(extractor *tarExtractor, size int64, name string) error {
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

func writeEntry(extractor *tarExtractor, header *tar.Header, target string) error {
	err := dispatchWriteEntry(extractor, header, target)
	if err != nil {
		return fmt.Errorf(errFmtWriteEntry, err)
	}

	return nil
}

func dispatchWriteEntry(extractor *tarExtractor, header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return dispatchDirectoryTarget(target)
	case tar.TypeReg:
		return dispatchRegularTarget(extractor, header, target)
	case tar.TypeSymlink, tar.TypeLink:
		return fmt.Errorf("link entry: %w", linkEntryError(header.Name))
	default:
		return fmt.Errorf("unsupported entry: %w", unsupportedEntryError(header.Name))
	}
}

func dispatchDirectoryTarget(target string) error {
	err := writeDirTarget(target)
	if err != nil {
		return fmt.Errorf("write directory target: %w", err)
	}

	return nil
}

func dispatchRegularTarget(extractor *tarExtractor, header *tar.Header, target string) error {
	err := writeRegTarget(extractor, header, target)
	if err != nil {
		return fmt.Errorf("write regular target: %w", err)
	}

	return nil
}

func writeRegEntry(extractor *tarExtractor, header *tar.Header, target string) error {
	err := validateSize(extractor, header.Size, header.Name)
	if err != nil {
		return fmt.Errorf("validate size: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(target), dirPerm)
	if err != nil {
		return fmt.Errorf("create parent directory for %q: %w", target, err)
	}

	err = writeRegularFile(
		extractor,
		&writeRegularArgs{path: target, mode: header.Mode, size: header.Size},
	)
	if err != nil {
		return fmt.Errorf("write regular file: %w", err)
	}

	return nil
}

func writeRegTarget(extractor *tarExtractor, header *tar.Header, target string) error {
	err := writeRegEntry(extractor, header, target)
	if err != nil {
		return fmt.Errorf("write reg entry: %w", err)
	}

	return nil
}

func writeRegularFile(extractor *tarExtractor, args *writeRegularArgs) error {
	cleanPath := filepath.Clean(args.path)

	file, err := openTarget(cleanPath, args.mode)
	if err != nil {
		return fmt.Errorf("open target: %w", err)
	}

	err = copyAndClose(extractor, &copyCloseArgs{file: file, size: args.size, path: cleanPath})
	if err != nil {
		return fmt.Errorf("copy and close: %w", err)
	}

	return nil
}

func writeResolvedEntry(extractor *tarExtractor, header *tar.Header, rel string) error {
	target := filepath.Join(extractor.destDir, rel)

	err := ensureInside(extractor.destDir, target)
	if err != nil {
		return fmt.Errorf("ensure inside: %w", err)
	}

	err = writeEntry(extractor, header, target)
	if err != nil {
		return fmt.Errorf(errFmtWriteEntry, err)
	}

	return nil
}
