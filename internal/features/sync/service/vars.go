// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

type fileOps struct {
	removeAll        func(string) error
	removePath       func(string) error
	mkdirAll         func(string, os.FileMode) error
	mkdirTemp        func(string, string) (string, error)
	createTemp       func(string, string) (*os.File, error)
	renamePath       func(string, string) error
	statPath         func(string) (os.FileInfo, error)
	walkDir          func(string, fs.WalkDirFunc) error
	writeFull        func(io.Writer, []byte) error
	readAll          func(io.Reader) ([]byte, error)
	openRelativeFile func(string, string) (*os.File, error)
	relPath          func(string, string) (string, error)
	closeFile        func(*os.File) error
	chmodFile        func(*os.File, os.FileMode) error
	marshalYAML      func(any) ([]byte, error)
}

func defaultFileOps() fileOps {
	return fileOps{
		removeAll:        os.RemoveAll,
		removePath:       os.Remove,
		mkdirAll:         os.MkdirAll,
		mkdirTemp:        os.MkdirTemp,
		createTemp:       os.CreateTemp,
		renamePath:       os.Rename,
		statPath:         os.Stat,
		walkDir:          filepath.WalkDir,
		writeFull:        iox.WriteFull,
		readAll:          io.ReadAll,
		openRelativeFile: pathutil.OpenRelativeFile,
		relPath:          filepath.Rel,
		closeFile:        (*os.File).Close,
		chmodFile:        (*os.File).Chmod,
		marshalYAML:      yaml.Marshal,
	}
}

func withFileOps(ops fileOps) fileOps {
	defaults := defaultFileOps()

	ops = fillFileOpsOne(ops, defaults)
	ops = fillFileOpsTwo(ops, defaults)
	ops = fillFileOpsThree(ops, defaults)
	ops = fillFileOpsFour(ops, defaults)
	ops = fillFileOpsFive(ops, defaults)

	return ops
}

func fillFileOpsOne(ops, defaults fileOps) fileOps {
	if ops.removeAll == nil {
		ops.removeAll = defaults.removeAll
	}

	if ops.removePath == nil {
		ops.removePath = defaults.removePath
	}

	if ops.mkdirAll == nil {
		ops.mkdirAll = defaults.mkdirAll
	}

	return ops
}

func fillFileOpsTwo(ops, defaults fileOps) fileOps {
	if ops.mkdirTemp == nil {
		ops.mkdirTemp = defaults.mkdirTemp
	}

	if ops.createTemp == nil {
		ops.createTemp = defaults.createTemp
	}

	if ops.renamePath == nil {
		ops.renamePath = defaults.renamePath
	}

	return ops
}

func fillFileOpsThree(ops, defaults fileOps) fileOps {
	if ops.statPath == nil {
		ops.statPath = defaults.statPath
	}

	if ops.walkDir == nil {
		ops.walkDir = defaults.walkDir
	}

	if ops.writeFull == nil {
		ops.writeFull = defaults.writeFull
	}

	return ops
}

func fillFileOpsFour(ops, defaults fileOps) fileOps {
	if ops.readAll == nil {
		ops.readAll = defaults.readAll
	}

	if ops.openRelativeFile == nil {
		ops.openRelativeFile = defaults.openRelativeFile
	}

	if ops.relPath == nil {
		ops.relPath = defaults.relPath
	}

	return ops
}

func fillFileOpsFive(ops, defaults fileOps) fileOps {
	if ops.closeFile == nil {
		ops.closeFile = defaults.closeFile
	}

	if ops.chmodFile == nil {
		ops.chmodFile = defaults.chmodFile
	}

	if ops.marshalYAML == nil {
		ops.marshalYAML = defaults.marshalYAML
	}

	return ops
}

var (
	errPreviousMetadataNotFound = errors.New("previous metadata not found")
	errMetadataNotFound         = errors.New("metadata not found")

	errTaskfileOpsNotConfigured = errors.New("taskfile ops not configured")

	errUnsupportedStoreMetadataSchema = errors.New("unsupported metadata.yml schema")
)
