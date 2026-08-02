// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	fieldTasks         = "tasks"
	fieldTargetFolder  = "target-folder"
	fieldPath          = "path"
	taskNamePatternMsg = "invalid task name %q: must match ^[a-z0-9][a-z0-9-]*$"
)

var (
	windowsAbsPath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	taskNameRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// PathError reports invalid path or task name configuration.
type PathError struct {
	Field   string
	Value   string
	Message string
}

func (e *PathError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}

	return e.Message
}

// NormalizeSlashes converts Windows separators and trims redundant slashes.
func NormalizeSlashes(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")

	path = strings.TrimSpace(path)

	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	path = strings.Trim(path, "/")

	return path
}

// ValidateTaskName checks a task name for safe characters and format.
func ValidateTaskName(name string) error {
	if name == "" {
		return &PathError{Field: fieldTasks, Value: "", Message: "task name must not be empty"}
	}

	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return &PathError{
			Field:   fieldTasks,
			Value:   name,
			Message: fmt.Sprintf("unsafe task name %q", name),
		}
	}

	if !taskNameRe.MatchString(name) {
		return &PathError{
			Field:   fieldTasks,
			Value:   name,
			Message: fmt.Sprintf(taskNamePatternMsg, name),
		}
	}

	return nil
}

// ValidateTargetFolder resolves and validates a workspace-relative target folder.
func ValidateTargetFolder(raw, workspace string) (string, error) {
	raw = strings.TrimSpace(raw)

	if raw == "" {
		return "", &PathError{Field: fieldTargetFolder, Value: "", Message: "must not be empty"}
	}

	normalized, err := normalizeTargetFolder(raw)
	if err != nil {
		return "", err
	}

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}

	evalWorkspace, err := filepath.EvalSymlinks(absWorkspace)
	if err != nil {
		evalWorkspace = absWorkspace
	}

	err = ensureTargetInsideWorkspace(evalWorkspace, normalized, raw)
	if err != nil {
		return "", err
	}

	err = validatePathComponents(evalWorkspace, normalized, raw)
	if err != nil {
		return "", err
	}

	return normalized, nil
}

func normalizeTargetFolder(raw string) (string, error) {
	if filepath.IsAbs(raw) || strings.HasPrefix(raw, "/") || windowsAbsPath.MatchString(raw) {
		return "", &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "must be a relative path",
		}
	}

	normalized := NormalizeSlashes(raw)

	if normalized == "" {
		return "", &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "must not be empty after normalization",
		}
	}

	if slices.Contains(strings.Split(normalized, "/"), "..") {
		return "", &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "must not contain .. path components",
		}
	}

	if normalized == ".git" || strings.HasPrefix(normalized, ".git/") {
		return "", &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "must not point to .git",
		}
	}

	if normalized == ".github/actions" || strings.HasPrefix(normalized, ".github/actions/") {
		return "", &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "must not point inside .github/actions",
		}
	}

	return normalized, nil
}

func ensureTargetInsideWorkspace(evalWorkspace, normalized, raw string) error {
	targetAbs := filepath.Join(evalWorkspace, filepath.FromSlash(normalized))

	rel, err := filepath.Rel(evalWorkspace, filepath.Clean(targetAbs))
	if err != nil {
		return &PathError{Field: fieldTargetFolder, Value: raw, Message: err.Error()}
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "path resolves outside workspace",
		}
	}

	return nil
}

func validatePathComponents(evalWorkspace, normalized, raw string) error {
	current := evalWorkspace

	for part := range strings.SplitSeq(normalized, "/") {
		current = filepath.Join(current, part)

		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return &PathError{Field: fieldTargetFolder, Value: raw, Message: err.Error()}
		}

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := filepath.EvalSymlinks(current)
			if err != nil {
				return &PathError{
					Field:   fieldTargetFolder,
					Value:   raw,
					Message: "invalid symlink target",
				}
			}

			linkRel, err := filepath.Rel(evalWorkspace, linkTarget)

			if err != nil || linkRel == ".." ||
				strings.HasPrefix(linkRel, ".."+string(os.PathSeparator)) {

				return &PathError{
					Field:   fieldTargetFolder,
					Value:   raw,
					Message: "must not escape through symlinks",
				}
			}

			current = linkTarget
		}
	}

	return nil
}

// JoinRelative joins path parts under base using normalized forward slashes.
func JoinRelative(base string, parts ...string) string {
	all := append([]string{base}, parts...)

	return NormalizeSlashes(filepath.ToSlash(filepath.Join(toOSParts(all)...)))
}

func toOSParts(parts []string) []string {
	out := make([]string, len(parts))

	for i, part := range parts {
		out[i] = filepath.FromSlash(part)
	}

	return out
}

// WorkspacePath joins workspace and a slash-separated relative path.
func WorkspacePath(workspace, rel string) string {
	return filepath.Join(workspace, filepath.FromSlash(rel))
}

// ValidateRelativePath checks that rel resolves to a path inside root.
func ValidateRelativePath(root, rel string) (string, error) {
	normalized, err := normalizeRelativePath(rel)
	if err != nil {
		return "", err
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}

	targetAbs := filepath.Join(absRoot, filepath.FromSlash(normalized))

	relToRoot, err := filepath.Rel(absRoot, filepath.Clean(targetAbs))
	if err != nil {
		return "", &PathError{Field: fieldPath, Value: rel, Message: err.Error()}
	}

	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) {
		return "", &PathError{Field: fieldPath, Value: rel, Message: "path resolves outside root"}
	}

	return normalized, nil
}

func normalizeRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)

	if rel == "" {
		return "", &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: "relative path must not be empty",
		}
	}

	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || windowsAbsPath.MatchString(rel) {
		return "", &PathError{Field: fieldPath, Value: rel, Message: "must be a relative path"}
	}

	normalized := NormalizeSlashes(rel)

	if normalized == "" {
		return "", &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: "must not be empty after normalization",
		}
	}

	if slices.Contains(strings.Split(normalized, "/"), "..") {
		return "", &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: "must not contain .. path components",
		}
	}

	return normalized, nil
}

// ReadRelativeFile reads rel under root after validating it stays inside root.
func ReadRelativeFile(root, rel string) ([]byte, error) {
	safeRel, err := ValidateRelativePath(root, rel)
	if err != nil {
		return nil, err
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	data, err := fs.ReadFile(os.DirFS(absRoot), safeRel)
	if err != nil {
		return nil, fmt.Errorf("read file %q: %w", rel, err)
	}

	return data, nil
}

// OpenRelativeFile opens rel under root after validating it stays inside root.
func OpenRelativeFile(root, rel string) (fs.File, error) {
	safeRel, err := ValidateRelativePath(root, rel)
	if err != nil {
		return nil, err
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	file, err := os.DirFS(absRoot).Open(safeRel)
	if err != nil {
		return nil, fmt.Errorf("open file %q: %w", rel, err)
	}

	return file, nil
}

// IsDocPath reports whether rel is documentation copied when includes-doc is enabled.
func IsDocPath(rel string) bool {
	rel = NormalizeSlashes(rel)

	if rel == "README.md" {
		return true
	}

	return strings.HasPrefix(rel, "docs/") || strings.Contains(rel, "/docs/")
}

// IsModuleMetadataPath reports whether rel is a store module's metadata.yml.
// It describes the module to the store and is not consumed by TaskOtter or the
// consumer repository, so it is never synced.
func IsModuleMetadataPath(rel string) bool {
	return NormalizeSlashes(rel) == "metadata.yml"
}

// IsTestPath reports whether rel is a test file skipped during sync.
func IsTestPath(rel string) bool {
	rel = NormalizeSlashes(rel)

	base := rel

	if idx := strings.LastIndex(rel, "/"); idx >= 0 {
		base = rel[idx+1:]
	}

	return strings.Contains(base, "_test.")
}

// HasFolderPrefix reports whether path is folder or a child path of folder.
func HasFolderPrefix(path, folder string) bool {
	path = NormalizeSlashes(path)

	folder = NormalizeSlashes(folder)

	if path == folder {
		return true
	}

	return strings.HasPrefix(path, folder+"/")
}
