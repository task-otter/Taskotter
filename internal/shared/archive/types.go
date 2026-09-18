// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package archive

import (
	"archive/tar"
	"io"
	"os"
)

type (
	// Reader is a readable archive byte stream.
	Reader interface {
		Read(p []byte) (n int, err error)
	}

	// ExtractError reports a safe-extraction failure.
	ExtractError struct {
		Message string
	}

	tarExtractor = struct {
		reader     *tar.Reader
		destDir    string
		rootPrefix string
		total      int64
		rootSet    bool
	}

	copyCloseArgs = struct {
		file *os.File
		path string
		size int64
	}

	copyLimArgs = struct {
		dst  io.Writer
		path string
		size int64
	}

	writeRegularArgs = struct {
		path string
		mode int64
		size int64
	}
)
