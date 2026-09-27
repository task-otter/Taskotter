// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"github.com/task-otter/Taskotter/internal/features/root/rootupd"
	yaml "go.yaml.in/yaml/v3"
)

type (
	opsFns = struct {
		newRoot    func() []byte
		rewrite    func([]byte, map[string]string, string) ([]byte, error)
		updateRoot func([]byte, *rootupd.RootUpdateInput) ([]byte, error)
	}

	// Ops describes the ops.
	Ops struct {
		fns opsFns
	}

	// RewriteError describes the rewrite \1rror.
	RewriteError struct {
		Message string
	}

	rootUpdateInput   = rootupd.RootUpdateInput
	generatedRootTask = rootupd.GeneratedRootTask

	marshalRootParams struct {
		node    *yaml.Node
		root    *yaml.Node
		input   *rootUpdateInput
		content []byte
	}

	rawBlockVarParams struct {
		out     map[string]string
		key     *yaml.Node
		value   *yaml.Node
		content []byte
	}

	includesUpdateParams = struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		moduleVars   map[string]*yaml.Node
		input        *rootUpdateInput
	}

	includeUpsertParams = struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		moduleVars   map[string]*yaml.Node
		input        *rootUpdateInput
		task         string
	}

	existingIncludeParams struct {
		entry        *yaml.Node
		moduleVars   *yaml.Node
		path         string
		dir          string
		task         string
		managedTasks []string
	}

	managedIncludeParams struct {
		entry        *yaml.Node
		expectedPath string
		task         string
		managedTasks []string
	}

	pruneIncludesParams struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		managedSet   map[string]struct{}
		managedTasks []string
	}

	promotedVarParams struct {
		root         *yaml.Node
		moduleVars   map[string]*yaml.Node
		rawVars      map[string]string
		promotedVars map[string]struct{}
		tasks        []string
	}

	addPromotedVarParams struct {
		rootVars   *yaml.Node
		moduleVars map[string]*yaml.Node
		rawVars    map[string]string
		existing   map[string]struct{}
		key        string
		tasks      []string
	}

	rootVarsResult struct {
		byTask yamlNodeMap
		raw    map[string]string
	}

	extractedVars struct {
		node *yaml.Node
		raw  map[string]string
		ok   bool
	}

	mergeModuleVarParams struct {
		existingVars *yaml.Node
		key          string
	}

	yamlNodeMap = map[string]*yaml.Node
	strSet      = map[string]struct{}
	rootUpdIn   = rootUpdateInput

	rewriteParams struct {
		path         string
		sourceToDest map[string]string
		fromDest     string
		dir          string
	}

	includePathReplacement struct {
		oldPath string
		newPath string
		line    int
		column  int
		style   yaml.Style
	}

	includePathSpan struct {
		value string
		start int
		end   int
	}

	rewriteIncludesParams struct {
		root         *yaml.Node
		sourceToDest map[string]string
		fromDest     string
		content      []byte
	}

	yamlPosition struct {
		content []byte
		line    int
		column  int
	}

	scalarSpanParams struct {
		oldPath string
		content []byte
		offset  int
		style   yaml.Style
	}

	quotedSpanParams struct {
		oldPath string
		content []byte
		offset  int
		quote   byte
	}

	replaceSpanParams struct {
		value   string
		content []byte
		start   int
		end     int
	}

	collectIncludeReplacementsParams = struct {
		includes     *yaml.Node
		entry        *yaml.Node
		sourceToDest map[string]string
		fromDest     string
		out          []includePathReplacement
	}
)
