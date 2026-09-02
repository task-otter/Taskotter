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
	//nolint:gochecknoglobals // test seams for OS failure branches
	removeAll = os.RemoveAll

	//nolint:gochecknoglobals // test seams for OS failure branches
	removePath = os.Remove

	//nolint:gochecknoglobals // test seams for OS failure branches
	mkdirAll = os.MkdirAll

	//nolint:gochecknoglobals // test seams for OS failure branches
	mkdirTemp = os.MkdirTemp

	//nolint:gochecknoglobals // test seams for OS failure branches
	createTemp = os.CreateTemp

	//nolint:gochecknoglobals // test seams for OS failure branches
	renamePath = os.Rename

	//nolint:gochecknoglobals // test seams for OS failure branches
	statPath = os.Stat

	//nolint:gochecknoglobals // test seams for OS failure branches
	walkDir = filepath.WalkDir

	//nolint:gochecknoglobals // test seams for I/O failure branches
	writeFull = iox.WriteFull

	//nolint:gochecknoglobals // test seams for I/O failure branches
	readAll = io.ReadAll

	//nolint:gochecknoglobals // test seams for path open failure branches
	openRelativeFile = pathutil.OpenRelativeFile

	//nolint:gochecknoglobals // test seams for path relative failure branches
	relPath = filepath.Rel

	//nolint:gochecknoglobals // test seams for file close failure branches
	closeFile = (*os.File).Close

	//nolint:gochecknoglobals // test seams for chmod failure branches
	chmodFile = (*os.File).Chmod

	//nolint:gochecknoglobals // test seams for YAML marshal failure branches
	marshalYAML = yaml.Marshal
)
