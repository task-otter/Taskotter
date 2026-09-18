// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package yamlutil

import (
	"slices"

	yaml "go.yaml.in/yaml/v3"
)

// New creates an empty YAML node of kind.
func New(kind yaml.Kind) *yaml.Node { return &yaml.Node{Kind: kind} }

// Mapping creates an empty mapping node.
func Mapping() *yaml.Node { return New(yaml.MappingNode) }

// Sequence creates an empty sequence node.
func Sequence() *yaml.Node { return New(yaml.SequenceNode) }

// Scalar creates a scalar node with value.
func Scalar(value string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: value} }

// AppendMappingPair appends one key/value pair to a mapping node.
func AppendMappingPair(mapping, key, value *yaml.Node) {
	mapping.Content = append(mapping.Content, key, value)
}

// Clone returns a deep copy of node.
func Clone(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	clone := *node

	clone.Content = make([]*yaml.Node, zeroIndex, len(node.Content))

	for index := range node.Content {
		clone.Content = append(clone.Content, Clone(node.Content[index]))
	}

	return &clone
}

// Keys returns the keys in a mapping node.
func Keys(mapping *yaml.Node) map[string]struct{} {
	keys := make(map[string]struct{}, len(mapping.Content)/mappingPairWidth)

	for index := zeroIndex; index < len(mapping.Content); index += mappingPairWidth {
		keys[mapping.Content[index].Value] = struct{}{}
	}

	return keys
}

// SetValue replaces or appends a mapping value.
func SetValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for index := zeroIndex; index < len(mapping.Content); index += mappingPairWidth {
		if mapping.Content[index].Value == key {
			mapping.Content[index+firstIndex] = value

			return
		}
	}

	AppendMappingPair(mapping, Scalar(key), value)
}

// DeleteKey removes one mapping key when present.
func DeleteKey(mapping *yaml.Node, key string) {
	for index := zeroIndex; index < len(mapping.Content); index += mappingPairWidth {
		if mapping.Content[index].Value == key {
			mapping.Content = append(
				mapping.Content[:index],
				mapping.Content[index+mappingPairWidth:]...)

			return
		}
	}
}

// SortedKeys returns keys in deterministic order.
func SortedKeys(keys map[string]struct{}) []string {
	result := make([]string, zeroIndex, len(keys))

	for key := range keys {
		result = append(result, key)
	}

	slices.Sort(result)

	return result
}
