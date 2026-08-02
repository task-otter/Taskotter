// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlfmt

import (
	"bytes"
	"fmt"

	yaml "go.yaml.in/yaml/v3"
)

const (
	// IndentSpaces is the two-space indentation used for all generated YAML.
	indentSpaces = 2
	// DocumentStart is the yamllint-required document-start marker.
	documentStart = "---\n"
)

// Marshal encodes value as a single YAML document prefixed with "---" using
// two-space indentation. Value may be a struct, map, or *yaml.Node. The result
// always ends with exactly one newline and carries exactly one document-start
// marker.
func Marshal(value any) ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indentSpaces)

	err := enc.Encode(value)
	if err != nil {
		_ = enc.Close()

		return nil, fmt.Errorf("encode yaml document: %w", err)
	}

	err = enc.Close()
	if err != nil {
		return nil, fmt.Errorf("close yaml encoder: %w", err)
	}

	// The encoder does not emit a leading "---" for the first document, but
	// strip one defensively so the output carries exactly one marker.
	body := bytes.TrimPrefix(buf.Bytes(), []byte(documentStart))

	out := make([]byte, 0, len(documentStart)+len(body))

	out = append(out, documentStart...)
	out = append(out, body...)

	return out, nil
}
