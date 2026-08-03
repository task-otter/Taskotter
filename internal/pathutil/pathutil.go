// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package pathutil provides safe path validation and normalization helpers.
package pathutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/consts"
)

// PathError reports invalid path or task name configuration.
type PathError struct {
	Field   string
	Value   string
	Message string
}

const (
	fieldTasks         = "tasks"
	fieldTargetFolder  = "target-folder"
	fieldPath          = "path"
	taskNamePatternMsg = "invalid task name %q: must match ^[a-z0-9][a-z0-9-]*$"

	errMustBeRelativePath      = "must be a relative path"
	errMustNotBeEmptyAfterNorm = "must not be empty after normalization"
	errMustNotContainDotDot    = "must not contain .. path components"
	errResolveRoot             = "resolve root: %w"
)

var (
	windowsAbsPath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)
	taskNameRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

// Error implements the error interface, returning the field-prefixed path error message.
func (e *PathError) Error() string {
	if e.Field != consts.Empty {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}

	return e.Message
}

// NormalizeSlashes converts Windows separators and trims redundant slashes.
func NormalizeSlashes(path string) string {
	path = strings.ReplaceAll(path, "\\", consts.PathSepString)

	path = strings.TrimSpace(path)

	for strings.Contains(path, consts.DoubleSlash) {
		path = strings.ReplaceAll(path, consts.DoubleSlash, consts.PathSepString)
	}

	path = strings.Trim(path, consts.PathSepString)

	return path
}

// ValidateTaskName checks a task name for safe characters and format.
func ValidateTaskName(name string) error {
	if name == consts.Empty {
		return &PathError{
			Field:   fieldTasks,
			Value:   consts.Empty,
			Message: "task name must not be empty",
		}
	}

	if strings.ContainsAny(name, "/\\") || strings.Contains(name, consts.PathParent) {
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

// isAbsoluteLikePath reports whether s looks like an absolute or drive-rooted path.
func isAbsoluteLikePath(s string) bool {
	return filepath.IsAbs(s) || strings.HasPrefix(s, consts.PathSepString) ||
		windowsAbsPath.MatchString(s)
}

// containsDotDotComponent reports whether normalized contains a ".." path component.
func containsDotDotComponent(normalized string) bool {
	return slices.Contains(strings.Split(normalized, consts.PathSepString), consts.PathParent)
}

// resolvesOutsideBase reports whether rel (from [filepath.Rel]) escapes its base directory.
func resolvesOutsideBase(rel string) bool {
	return rel == consts.PathParent ||
		strings.HasPrefix(rel, consts.PathParent+string(os.PathSeparator))
}

// ValidateTargetFolder resolves and validates a workspace-relative target folder.
func ValidateTargetFolder(raw, workspace string) (string, error) {
	trimmed, normalized, err := normalizeAndValidateTargetFolder(raw)
	if err != nil {
		return consts.Empty, fmt.Errorf("normalize target folder: %w", err)
	}

	evalWorkspace, err := resolveEvalWorkspace(workspace)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve evaluation workspace: %w", err)
	}

	err = ensureSafeTargetFolder(evalWorkspace, normalized, trimmed)
	if err != nil {
		return consts.Empty, fmt.Errorf("ensure safe target folder: %w", err)
	}

	return normalized, nil
}

func normalizeAndValidateTargetFolder(raw string) (trimmedOut, normalizedOut string, err error) {
	trimmed := strings.TrimSpace(raw)

	if trimmed == consts.Empty {
		return trimmed, consts.Empty, &PathError{
			Field:   fieldTargetFolder,
			Value:   consts.Empty,
			Message: "must not be empty",
		}
	}

	normalized, err := normalizeTargetFolder(trimmed)
	if err != nil {
		return trimmed, consts.Empty, fmt.Errorf("normalize target folder: %w", err)
	}

	return trimmed, normalized, nil
}

func resolveEvalWorkspace(workspace string) (string, error) {
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve workspace: %w", err)
	}

	evalWorkspace, err := filepath.EvalSymlinks(absWorkspace)
	if err != nil {
		evalWorkspace = absWorkspace
	}

	return evalWorkspace, nil
}

func ensureSafeTargetFolder(evalWorkspace, normalized, raw string) error {
	err := ensureTargetInsideWorkspace(evalWorkspace, normalized, raw)
	if err != nil {
		return fmt.Errorf("ensure target inside workspace: %w", err)
	}

	err = validatePathComponents(evalWorkspace, normalized, raw)
	if err != nil {
		return fmt.Errorf("validate path components: %w", err)
	}

	return nil
}

func normalizeTargetFolder(raw string) (string, error) {
	if isAbsoluteLikePath(raw) {
		return consts.Empty, &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: errMustBeRelativePath,
		}
	}

	normalized := NormalizeSlashes(raw)

	if normalized == consts.Empty {
		return consts.Empty, &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: errMustNotBeEmptyAfterNorm,
		}
	}

	err := validateTargetFolderComponents(normalized, raw)
	if err != nil {
		return consts.Empty, fmt.Errorf("validate target folder components: %w", err)
	}

	return normalized, nil
}

func validateTargetFolderComponents(normalized, raw string) error {
	if containsDotDotComponent(normalized) {
		return &PathError{Field: fieldTargetFolder, Value: raw, Message: errMustNotContainDotDot}
	}

	if pointsIntoGitDir(normalized) {
		return &PathError{Field: fieldTargetFolder, Value: raw, Message: "must not point to .git"}
	}

	if pointsIntoGitHubActions(normalized) {
		return &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "must not point inside .github/actions",
		}
	}

	return nil
}

func pointsIntoGitDir(normalized string) bool {
	return normalized == consts.GitDir || strings.HasPrefix(normalized, consts.GitDirWithSep)
}

func pointsIntoGitHubActions(normalized string) bool {
	return normalized == consts.GitHubActions ||
		strings.HasPrefix(normalized, consts.GitHubActsWith)
}

func ensureTargetInsideWorkspace(evalWorkspace, normalized, raw string) error {
	targetAbs := filepath.Join(evalWorkspace, filepath.FromSlash(normalized))

	rel, err := filepath.Rel(evalWorkspace, filepath.Clean(targetAbs))
	if err != nil {
		return &PathError{Field: fieldTargetFolder, Value: raw, Message: err.Error()}
	}

	if resolvesOutsideBase(rel) {
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

	for part := range strings.SplitSeq(normalized, consts.PathSepString) {
		next, err := processPathComponent(evalWorkspace, filepath.Join(current, part), raw)
		if err != nil {
			return fmt.Errorf("process path component %q: %w", part, err)
		}

		current = next
	}

	return nil
}

func processPathComponent(evalWorkspace, current, raw string) (string, error) {
	info, exists, err := statPathComponent(current, raw)
	if err != nil {
		return consts.Empty, fmt.Errorf("stat path component: %w", err)
	}

	if !exists || info.Mode()&os.ModeSymlink == consts.IndexZero {
		return current, nil
	}

	resolved, err := resolveSymlinkComponent(evalWorkspace, current, raw)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve symlink component: %w", err)
	}

	return resolved, nil
}

func statPathComponent(current, raw string) (info os.FileInfo, exists bool, err error) {
	info, err = os.Lstat(current)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}

		return nil, false, &PathError{Field: fieldTargetFolder, Value: raw, Message: err.Error()}
	}

	return info, true, nil
}

func resolveSymlinkComponent(evalWorkspace, current, raw string) (string, error) {
	linkTarget, err := filepath.EvalSymlinks(current)
	if err != nil {
		return consts.Empty, &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "invalid symlink target",
		}
	}

	linkRel, err := filepath.Rel(evalWorkspace, linkTarget)

	if err != nil || resolvesOutsideBase(linkRel) {
		return consts.Empty, &PathError{
			Field:   fieldTargetFolder,
			Value:   raw,
			Message: "must not escape through symlinks",
		}
	}

	return linkTarget, nil
}

// JoinRelative joins path parts under base using normalized forward slashes.
func JoinRelative(base string, parts ...string) string {
	all := append([]string{base}, parts...)

	return NormalizeSlashes(filepath.ToSlash(filepath.Join(toOSParts(all)...)))
}

func toOSParts(parts []string) []string {
	out := make([]string, len(parts))

	for i := range parts {
		out[i] = filepath.FromSlash(parts[i])
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
		return consts.Empty, fmt.Errorf("normalize relative path: %w", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return consts.Empty, fmt.Errorf(errResolveRoot, err)
	}

	err = ensureRelativeInsideRoot(absRoot, normalized, rel)
	if err != nil {
		return consts.Empty, fmt.Errorf("ensure relative path inside root: %w", err)
	}

	return normalized, nil
}

func ensureRelativeInsideRoot(absRoot, normalized, rel string) error {
	targetAbs := filepath.Join(absRoot, filepath.FromSlash(normalized))

	relToRoot, err := filepath.Rel(absRoot, filepath.Clean(targetAbs))
	if err != nil {
		return &PathError{Field: fieldPath, Value: rel, Message: err.Error()}
	}

	if resolvesOutsideBase(relToRoot) {
		return &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: "path resolves outside root",
		}
	}

	return nil
}

func normalizeRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)

	if rel == consts.Empty {
		return consts.Empty, &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: "relative path must not be empty",
		}
	}

	if isAbsoluteLikePath(rel) {
		return consts.Empty, &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: errMustBeRelativePath,
		}
	}

	normalized, err := normalizeAndCheckDotDot(rel)
	if err != nil {
		return consts.Empty, fmt.Errorf("check relative path for dot-dot: %w", err)
	}

	return normalized, nil
}

func normalizeAndCheckDotDot(rel string) (string, error) {
	normalized := NormalizeSlashes(rel)

	if normalized == consts.Empty {
		return consts.Empty, &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: errMustNotBeEmptyAfterNorm,
		}
	}

	if containsDotDotComponent(normalized) {
		return consts.Empty, &PathError{
			Field:   fieldPath,
			Value:   rel,
			Message: errMustNotContainDotDot,
		}
	}

	return normalized, nil
}

// ReadRelativeFile reads rel under root after validating it stays inside root.
func ReadRelativeFile(root, rel string) ([]byte, error) {
	safeRel, err := ValidateRelativePath(root, rel)
	if err != nil {
		return nil, fmt.Errorf("validate relative path %q: %w", rel, err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf(errResolveRoot, err)
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
		return nil, fmt.Errorf("validate relative path %q: %w", rel, err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf(errResolveRoot, err)
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

	if rel == consts.ReadmeMD {
		return true
	}

	return strings.HasPrefix(rel, "docs"+consts.PathSepString) ||
		strings.Contains(rel, consts.PathSepString+"docs"+consts.PathSepString)
}

// IsModuleMetadataPath reports whether rel is a store module's metadata.yml.
// It describes the module to the store and is not consumed by TaskOtter or the
// consumer repository, so it is never synced.
func IsModuleMetadataPath(rel string) bool {
	return NormalizeSlashes(rel) == consts.Metadata
}

// IsTestPath reports whether rel is a test file skipped during sync.
func IsTestPath(rel string) bool {
	rel = NormalizeSlashes(rel)

	base := rel

	if idx := strings.LastIndex(rel, consts.PathSepString); idx >= consts.IndexZero {
		base = rel[idx+consts.IndexOne:]
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

	return strings.HasPrefix(path, folder+consts.PathSepString)
}
