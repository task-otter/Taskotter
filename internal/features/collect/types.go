// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package collect

import (
	"os"
)

type (
	// File describes a file selected for synchronization.
	File struct {
		RelativePath string
		Data         []byte
		Mode         os.FileMode
	}

	// Options controls file collection.
	Options struct {
		SourceDir    string
		SourceToDest map[string]string
		FromDest     string
		IncludeDocs  bool
	}

	// Collector collects files from a module.
	Collector interface {
		Collect(Options) ([]File, error)
	}

	// DefaultCollector is the standard filesystem-backed collector.
	DefaultCollector struct{}
)
