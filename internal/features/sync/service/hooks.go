// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"io"
	"os"
	"path/filepath"

	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

// Package-level FS seams let tests reach OS failure branches without depending on
// platform-specific permission tricks. Production keeps the stdlib defaults.
var (
	removeAll = os.RemoveAll

	removePath = os.Remove

	mkdirAll = os.MkdirAll

	mkdirTemp = os.MkdirTemp

	createTemp = os.CreateTemp

	renamePath = os.Rename

	statPath = os.Stat

	walkDir = filepath.WalkDir

	writeFull = iox.WriteFull

	readAll = io.ReadAll

	openRelativeFile = pathutil.OpenRelativeFile

	relPath = filepath.Rel

	closeFile = (*os.File).Close

	chmodFile = (*os.File).Chmod

	marshalYAML = yaml.Marshal
)
