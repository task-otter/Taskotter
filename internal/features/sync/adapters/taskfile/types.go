// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"github.com/task-otter/Taskotter/internal/features/sync/domain/rootupd"
	yaml "go.yaml.in/yaml/v3"
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

	rootTextEdit struct {
		replacement []byte
		start       int
		end         int
		order       int
	}

	rawModuleVars = map[string]map[string][]byte

	rootPatchParams struct {
		originalRoot *yaml.Node
		desiredRoot  *yaml.Node
		input        *rootUpdateInput
		rawVars      rawModuleVars
		content      []byte
	}

	renderSectionParams struct {
		section string
		node    *yaml.Node
		rawVars rawModuleVars
		tasks   []string
	}

	managedMappingParams struct {
		params   *rootPatchParams
		original *yaml.Node
		desired  *yaml.Node
		managed  map[string]struct{}
	}

	sourceNodeSpanParams struct {
		parent    *yaml.Node
		content   []byte
		pairIndex int
		parentEnd int
	}

	scalarNodeSpanParams struct {
		key     *yaml.Node
		value   *yaml.Node
		content []byte
		start   int
	}

	managedEntryParams struct {
		input      *managedMappingParams
		key        string
		pairIndex  int
		sectionEnd int
	}

	parsedRootTaskfileParams struct {
		input   *rootUpdateInput
		node    *yaml.Node
		root    *yaml.Node
		content []byte
	}

	rootSectionReplacementParams struct {
		original mappingPair
		desired  mappingPair
		params   *rootPatchParams
		section  string
	}

	rootSectionEditParams struct {
		params   *rootPatchParams
		original *yaml.Node
		desired  *yaml.Node
		section  string
	}

	mappingEntrySpanParams struct {
		parent     *yaml.Node
		content    []byte
		pairIndex  int
		sectionEnd int
	}

	mappingPair struct {
		key   *yaml.Node
		value *yaml.Node
	}

	// RewriteError reports Taskfile YAML rewrite failures.
	RewriteError struct {
		Message string
	}

	rootUpdateInput   = rootupd.RootUpdateInput
	generatedRootTask = rootupd.GeneratedRootTask

	// includesUpdateParams carries state for merging managed includes into the root Taskfile.
	includesUpdateParams = struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		moduleVars   map[string]*yaml.Node
		input        *rootUpdateInput
	}

	// includeUpsertParams carries state for upserting one managed include entry.
	includeUpsertParams = struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		moduleVars   map[string]*yaml.Node
		input        *rootUpdateInput
		task         string
	}

	// existingIncludeParams carries state for updating an existing include entry.
	existingIncludeParams struct {
		entry        *yaml.Node
		moduleVars   *yaml.Node
		path         string
		dir          string
		task         string
		managedTasks []string
	}

	// managedIncludeParams carries state for checking whether an include is managed.
	managedIncludeParams struct {
		entry        *yaml.Node
		expectedPath string
		task         string
		managedTasks []string
	}

	// pruneIncludesParams carries state for removing stale managed includes.
	pruneIncludesParams struct {
		includesNode *yaml.Node
		existing     map[string]*yaml.Node
		managedSet   map[string]struct{}
		managedTasks []string
	}

	// promotedVarParams carries state for promoting module vars to the root.
	promotedVarParams struct {
		root         *yaml.Node
		moduleVars   map[string]*yaml.Node
		promotedVars map[string]struct{}
		tasks        []string
	}

	// addPromotedVarParams carries state for adding one promoted var to the root.
	addPromotedVarParams struct {
		rootVars   *yaml.Node
		moduleVars map[string]*yaml.Node
		existing   map[string]struct{}
		key        string
		tasks      []string
	}

	// mergeModuleVarParams carries state for merging one module var into an include.
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

	// includePathReplacement locates one include taskfile scalar in the original YAML.
	includePathReplacement struct {
		oldPath string
		newPath string
		line    int
		column  int
		style   yaml.Style
	}

	// includePathSpan is a byte range in the original content to overwrite.
	includePathSpan struct {
		value string
		start int
		end   int
	}

	// rewriteIncludesParams carries state for rewriting include paths in a Taskfile.
	rewriteIncludesParams struct {
		root         *yaml.Node
		sourceToDest map[string]string
		fromDest     string
		content      []byte
	}

	// yamlPosition locates a scalar at a 1-based YAML line/column in content.
	yamlPosition struct {
		content []byte
		line    int
		column  int
	}

	// scalarSpanParams locates the byte span of a scalar value at offset.
	scalarSpanParams struct {
		oldPath string
		content []byte
		offset  int
		style   yaml.Style
	}

	// quotedSpanParams locates the interior of a quoted scalar at offset.
	quotedSpanParams struct {
		oldPath string
		content []byte
		offset  int
		quote   byte
	}

	// replaceSpanParams overwrites content[start:end] with value.
	replaceSpanParams struct {
		value   string
		content []byte
		start   int
		end     int
	}

	// collectIncludeReplacementsParams carries state for collecting include path edits.
	collectIncludeReplacementsParams = struct {
		includes     *yaml.Node
		entry        *yaml.Node
		sourceToDest map[string]string
		fromDest     string
		out          []includePathReplacement
	}
)
