// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package collect

import (
	"slices"
	"strings"

	"github.com/task-otter/Taskotter/internal/features/plan"
)

// Collect implements Collector using the existing module traversal rules.
func (DefaultCollector) Collect(opts Options) ([]File, error) {
	contents, err := plan.CollectModuleFiles(&plan.CollectOptions{
		SourceDir:    opts.SourceDir,
		SourceToDest: opts.SourceToDest,
		FromDest:     opts.FromDest,
		DocPolicy:    docsPolicy(opts.IncludeDocs),
	})
	if err != nil {
		return nil, err
	}

	files := make([]File, 0, len(contents))

	for rel, entry := range contents {
		files = append(files, File{RelativePath: rel, Data: entry.Data, Mode: entry.Mode})
	}

	slices.SortFunc(files, func(a, b File) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})

	return files, nil
}

func docsPolicy(include bool) plan.DocPolicy {
	if include {
		return plan.DocPolicyInclude
	}

	return plan.DocPolicySkip
}
