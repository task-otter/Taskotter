// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"errors"
	"fmt"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/yamlfmt"
	yaml "go.yaml.in/yaml/v3"
)

type (
	yamlDecodeTarget struct {
		Out any
		Key string
	}
)

const (
	// YAMLMappingPairKeyValue is the stride between key and value nodes in a YAML mapping.
	YAMLMappingPairKeyValue = consts.IndexTwo

	errDecode                = "decode %q: %w"
	yamlKeyConfigurationHash = "configuration_hash"
	yamlKeyLockFile          = "lock_file"
	yamlKeyTargetFolder      = "target_folder"

	// YAMLKeyExportedTasks is the store metadata key for exported tasks.
	YAMLKeyExportedTasks = "exported_tasks"

	// YAMLKeyModule is the store metadata key for the module name.
	YAMLKeyModule = "module"

	// YAMLKeySchema is the store metadata key for the schema version.
	YAMLKeySchema = "schema"

	// YAMLKeyTaskfile is the store metadata key for the taskfile path.
	YAMLKeyTaskfile = "taskfile"

	// YAMLKeyVariants is the store metadata key for module variants.
	YAMLKeyVariants = "variants"
)

var errYAMLMappingNodeExpected = errors.New("expected YAML mapping node")

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

// UnmarshalYAML decodes TaskOtter metadata from its stable on-disk keys.
func (meta *Metadata) UnmarshalYAML(value *yaml.Node) error {
	err := UnmarshalYAMLMapping(value, "metadata", map[string]any{
		yamlKeyTargetFolder:      &meta.TargetFolder,
		yamlKeyLockFile:          &meta.LockFile,
		yamlKeyConfigurationHash: &meta.ConfigurationHash,
	})
	if err != nil {
		return fmt.Errorf("unmarshal metadata: %w", err)
	}

	return nil
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
