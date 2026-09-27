// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"fmt"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

// Error implements the error interface, returning the sync planning failure message.
func (err SyncError) Error() string {
	return string(err)
}

// MarshalMetadata encodes metadata using stable on-disk keys. Encoding a plain
// string map cannot fail, so no error is reported.
func MarshalMetadata(meta *Metadata) []byte {
	data, err := yamlfmt.Marshal(encodeMetadata(meta))
	iox.Discard(err)

	return data
}

// UnmarshalYAMLMapping decodes YAML mapping keys into destinations in outs (key -> pointer).
func UnmarshalYAMLMapping(value *yaml.Node, label string, outs map[string]any) error {
	targets := make([]yamlDecodeTarget, consts.IndexZero, len(outs))

	for key := range outs {
		targets = append(targets, yamlDecodeTarget{Key: key, Out: outs[key]})
	}

	err := unmarshalYAMLTargets(value, label, targets...)
	if err != nil {
		return fmt.Errorf("unmarshal YAML mapping %q: %w", label, err)
	}

	return nil
}

// UnmarshalMetadata decodes TaskOtter metadata from its stable on-disk keys.
func UnmarshalMetadata(value *yaml.Node, meta *Metadata) error {
	err := UnmarshalYAMLMapping(value, "metadata", map[string]any{
		yamlKeyTargetFolder:      &meta.TargetFolder,
		yamlKeyLockFile:          &meta.LockFile,
		yamlKeyConfigurationHash: &meta.ConfigurationHash,
	})
	if err != nil {
		return fmt.Errorf(errUnmarshalMetadataFmt, err)
	}

	return nil
}

// DecodeMetadataYAML unmarshals YAML bytes into metadata.
func DecodeMetadataYAML(data []byte, meta *Metadata) error {
	var node yaml.Node

	err := yaml.Unmarshal(data, &node)
	if err != nil {
		return fmt.Errorf("decode metadata yaml: %w", err)
	}

	err = UnmarshalMetadata(yamlDocumentContent(&node), meta)
	if err != nil {
		return fmt.Errorf(errUnmarshalMetadataFmt, err)
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

func encodeMetadata(meta *Metadata) map[string]string {
	return map[string]string{
		yamlKeyTargetFolder:      meta.TargetFolder,
		yamlKeyLockFile:          meta.LockFile,
		yamlKeyConfigurationHash: meta.ConfigurationHash,
	}
}

func unmarshalYAMLTargets(value *yaml.Node, label string, targets ...yamlDecodeTarget) error {
	fields, err := yamlFields(value)
	if err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}

	err = decodeYAMLFields(fields, targets...)
	if err != nil {
		return fmt.Errorf("decode %s fields: %w", label, err)
	}

	return nil
}

func yamlFields(value *yaml.Node) (map[string]*yaml.Node, error) {
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: got %v", errYAMLMappingNodeExpected, value.Kind)
	}

	fields := make(map[string]*yaml.Node, len(value.Content)/YAMLMappingPairKeyValue)

	for idx := consts.IndexZero; idx < len(value.Content); idx += YAMLMappingPairKeyValue {
		fields[value.Content[idx].Value] = value.Content[idx+consts.IndexOne]
	}

	return fields, nil
}
