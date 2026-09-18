// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt

const (
	// IndentSpaces is the two-space indentation used for all generated YAML.
	indentSpaces = 2
	// DocumentStart is the yamllint-required document-start marker.
	documentStart       = "---\n"
	errEncodeYAMLDoc    = "encode yaml document"
	errCloseYAMLEncoder = "close yaml encoder"
)
