// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
)

type (
	// Ops adapts package-level Taskfile helpers to ports.TaskfileOps.
	Ops struct{}
)

// NewRootTemplate returns the default root Taskfile template bytes.
func (Ops) NewRootTemplate() []byte {
	return NewRootTemplate()
}

// RewriteIncludes rewrites module include paths using sourceToDest.
func (Ops) RewriteIncludes(content []byte, sourceToDest map[string]string) ([]byte, error) {
	data, err := RewriteIncludes(content, sourceToDest)
	if err != nil {
		return nil, fmt.Errorf("rewrite includes: %w", err)
	}

	return data, nil
}

// UpdateRootTaskfile merges managed includes and generated tasks into the root Taskfile.
func (Ops) UpdateRootTaskfile(content []byte, input *rootupd.RootUpdateInput) ([]byte, error) {
	data, err := UpdateRootTaskfile(content, input)
	if err != nil {
		return nil, fmt.Errorf("update root taskfile: %w", err)
	}

	return data, nil
}
