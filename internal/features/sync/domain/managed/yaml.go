// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package managed

import (
	"errors"
	"fmt"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	yaml "go.yaml.in/yaml/v3"
)

type (
	yamlDecodeTarget = struct {
		Out any
		Key string
	}
)

const (
	errDecode                = "decode %q: %w"
	errUnmarshalManagedFmt   = "unmarshal managed file: %w"
	yamlKeyDestinationModule = "destination_module"
	yamlKeyPath              = "path"
	yamlKeySHA256            = "sha256"
	yamlKeySourceModule      = "source_module"
	yamlKeySourcePath        = "source_path"
	yamlMappingPairKeyValue  = consts.IndexTwo
)

var errYAMLMappingNodeExpected = errors.New("expected YAML mapping node")

// UnmarshalFile decodes a managed file from the lock file's snake_case keys.
func UnmarshalFile(value *yaml.Node, file *File) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode managed file: %w", err)
	}

	err = decodeYAMLFields(
		fields,
		yamlDecodeTarget{Key: yamlKeySourceModule, Out: &file.SourceModule},
		yamlDecodeTarget{Key: yamlKeyDestinationModule, Out: &file.DestinationModule},
		yamlDecodeTarget{Key: yamlKeySourcePath, Out: &file.SourcePath},
		yamlDecodeTarget{Key: yamlKeyPath, Out: &file.Path},
		yamlDecodeTarget{Key: yamlKeySHA256, Out: &file.SHA256},
	)
	if err != nil {
		return fmt.Errorf(errUnmarshalManagedFmt, err)
	}

	return nil
}

// DecodeFileYAML unmarshals YAML bytes into a managed file.
func DecodeFileYAML(data []byte, file *File) error {
	var node yaml.Node

	err := yaml.Unmarshal(data, &node)
	if err != nil {
		return fmt.Errorf("decode managed file yaml: %w", err)
	}

	err = UnmarshalFile(yamlDocumentContent(&node), file)
	if err != nil {
		return fmt.Errorf(errUnmarshalManagedFmt, err)
	}

	return nil
}

func yamlDocumentContent(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) > consts.IndexZero {
		return node.Content[consts.IndexZero]
	}

	return node
}

func decodeYAMLField(fields map[string]*yaml.Node, key string, out any) error {
	node := fields[key]

	if node == nil {
		return nil
	}

	err := node.Decode(out)
	if err != nil {
		return fmt.Errorf(errDecode, key, err)
	}

	return nil
}

func decodeYAMLFields(fields map[string]*yaml.Node, targets ...yamlDecodeTarget) error {
	for i := range targets {
		target := &targets[i]

		err := decodeYAMLField(fields, target.Key, target.Out)
		if err != nil {
			return fmt.Errorf("decode field %q: %w", target.Key, err)
		}
	}

	return nil
}

func yamlFields(value *yaml.Node) (map[string]*yaml.Node, error) {
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: got %v", errYAMLMappingNodeExpected, value.Kind)
	}

	fields := make(map[string]*yaml.Node, len(value.Content)/yamlMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(value.Content); idx += yamlMappingPairKeyValue {
		fields[value.Content[idx].Value] = value.Content[idx+consts.IndexOne]
	}

	return fields, nil
}
