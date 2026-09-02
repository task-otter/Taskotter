// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package yamlfmt marshals values to YAML with consistent document formatting.
package yamlfmt

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	yaml "go.yaml.in/yaml/v3"
)

const (
	// IndentSpaces is the two-space indentation used for all generated YAML.
	indentSpaces = 2
	// DocumentStart is the yamllint-required document-start marker.
	documentStart       = "---\n"
	errEncodeYAMLDoc    = "encode yaml document"
	errCloseYAMLEncoder = "close yaml encoder"
)

// Marshal encodes value as a single YAML document prefixed with "---" using
// two-space indentation. Value may be a struct, map, or *yaml.Node. The result
// always ends with exactly one newline and carries exactly one document-start
// marker.
func Marshal(value any) ([]byte, error) {
	body, err := encodeYAML(value)
	if err != nil {
		return nil, fmt.Errorf("encode yaml: %w", err)
	}

	return ensureSingleDocumentMarker(body), nil
}

func encodeYAML(value any) ([]byte, error) {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indentSpaces)

	err := encodeAndClose(enc, value)
	if err != nil {
		return nil, fmt.Errorf("encode and close yaml encoder: %w", err)
	}

	return buf.Bytes(), nil
}

func encodeAndClose(enc *yaml.Encoder, value any) error {
	encErr := enc.Encode(value)
	closeErr := enc.Close()

	return errors.Join(
		wrapNonNil(errEncodeYAMLDoc, encErr),
		wrapNonNil(errCloseYAMLEncoder, closeErr),
	)
}

func wrapNonNil(label string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", label, err)
}

// ensureSingleDocumentMarker strips any leading "---" the encoder may have emitted
// and prepends exactly one, since the encoder omits it for the first document but
// yamllint requires it.
func ensureSingleDocumentMarker(body []byte) []byte {
	body = bytes.TrimPrefix(body, []byte(documentStart))

	out := make([]byte, consts.IndexZero, len(documentStart)+len(body))

	out = append(out, documentStart...)
	out = append(out, body...)

	return out
}
