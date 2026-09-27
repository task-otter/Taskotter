// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package collect

import (
	"os"
)

type (
	File struct {
		RelativePath string
		Data         []byte
		Mode         os.FileMode
	}

	Options struct {
		SourceDir    string
		SourceToDest map[string]string
		FromDest     string
		IncludeDocs  bool
	}

	Collector interface {
		Collect(Options) ([]File, error)
	}

	DefaultCollector struct{}
)
