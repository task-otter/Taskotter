// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	escapingRelative = "../escape"
	wantErrMsg       = "expected error, got %v"
	wantEscapeMsg    = "expected escape rejection"
	taskfileName     = "Taskfile.yml"
)

var errAbsFailed = errors.New("abs failed")

// TestValidateTargetFolderReportsAbsFailure verifies an unresolvable workspace is reported.
//
//nolint:paralleltest // swaps the package-level absPath seam
func TestValidateTargetFolderReportsAbsFailure(t *testing.T) {
	failAbsPath(t)

	got, err := ValidateTargetFolder("taskfiles", "workspace")
	if err == nil {
		t.Fatalf(wantErrMsg, got)
	}
}

// TestValidateRelativePathReportsAbsFailure verifies an unresolvable root is reported.
//
//nolint:paralleltest // swaps the package-level absPath seam
func TestValidateRelativePathReportsAbsFailure(t *testing.T) {
	failAbsPath(t)

	got, err := ValidateRelativePath("root", taskfileName)
	if err == nil {
		t.Fatalf(wantErrMsg, got)
	}
}

// TestResolveValidatedRootReportsAbsFailure verifies the second abs call failure is reported.
//
//nolint:paralleltest // swaps the package-level absPath seam
func TestResolveValidatedRootReportsAbsFailure(t *testing.T) {
	root := t.TempDir()
	calls := consts.IndexZero

	swapAbsPath(t, func(path string) (string, error) {
		calls++

		if calls > consts.IndexOne {
			return consts.Empty, errAbsFailed
		}

		return filepath.Abs(path)
	})

	absRoot, safeRel, err := resolveValidatedRoot(root, taskfileName)
	iox.Discard2(absRoot, safeRel)

	if !errors.Is(err, errAbsFailed) {
		t.Fatalf("err = %v, want %v", err, errAbsFailed)
	}
}

// TestEnsureInsideRootRejectsEscape verifies a normalized path leaving the base is rejected.
func TestEnsureInsideRootRejectsEscape(t *testing.T) {
	t.Parallel()

	err := ensureInsideRoot(&insideRootParams{
		base:       t.TempDir(),
		normalized: escapingRelative,
		raw:        escapingRelative,
		field:      fieldPath,
		outsideMsg: errPathOutsideRootMsg,
	})
	if err == nil {
		t.Fatal(wantEscapeMsg)
	}
}

// TestEnsureSafeTargetFolderRejectsEscape verifies the target-folder guard rejects escapes.
func TestEnsureSafeTargetFolderRejectsEscape(t *testing.T) {
	t.Parallel()

	err := ensureSafeTargetFolder(t.TempDir(), escapingRelative, escapingRelative)
	if err == nil {
		t.Fatal(wantEscapeMsg)
	}
}

// TestValidateInsideRootRejectsEscape verifies the relative-path guard rejects escapes.
func TestValidateInsideRootRejectsEscape(t *testing.T) {
	t.Parallel()

	err := validateInsideRoot(t.TempDir(), escapingRelative, escapingRelative)
	if err == nil {
		t.Fatal(wantEscapeMsg)
	}
}

func failAbsPath(t *testing.T) {
	t.Helper()

	swapAbsPath(t, func(string) (string, error) {
		return consts.Empty, errAbsFailed
	})
}

func swapAbsPath(t *testing.T, stub func(string) (string, error)) {
	t.Helper()

	original := absPath

	absPath = stub

	t.Cleanup(func() { absPath = original })
}
