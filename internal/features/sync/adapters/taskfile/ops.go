// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
)

type (
	opsFns = struct {
		newRoot    func() []byte
		rewrite    func([]byte, map[string]string, string) ([]byte, error)
		updateRoot func([]byte, *rootupd.RootUpdateInput) ([]byte, error)
	}

	// Ops adapts package-level Taskfile helpers to ports.TaskfileOps.
	Ops struct {
		fns opsFns
	}
)

// NewOps returns an Ops wired to the package Taskfile helpers.
func NewOps() Ops {
	return Ops{fns: opsFns{
		newRoot:    NewRootTemplate,
		rewrite:    RewriteIncludes,
		updateRoot: UpdateRootTaskfile,
	}}
}

// NewRootTemplate returns the default root Taskfile template bytes.
func (ops Ops) NewRootTemplate() []byte {
	return ops.fns.newRoot()
}

// RewriteIncludes rewrites module include paths using sourceToDest and fromDest.
func (ops Ops) RewriteIncludes(
	content []byte,
	sourceToDest map[string]string,
	fromDest string,
) ([]byte, error) {
	data, err := ops.fns.rewrite(content, sourceToDest, fromDest)
	if err != nil {
		return nil, fmt.Errorf("rewrite includes: %w", err)
	}

	return data, nil
}

// UpdateRootTaskfile merges managed includes and generated tasks into the root Taskfile.
func (ops Ops) UpdateRootTaskfile(content []byte, input *ports.RootUpdateInput) ([]byte, error) {
	data, err := ops.fns.updateRoot(content, input)
	if err != nil {
		return nil, fmt.Errorf("update root taskfile: %w", err)
	}

	return data, nil
}
