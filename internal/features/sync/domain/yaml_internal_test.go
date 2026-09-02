// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	yaml "go.yaml.in/yaml/v3"
)

type (
	metaYAMLCase struct {
		name    string
		payload string
	}
)

const (
	wantErrText   = "expected error"
	syncErrText   = "sync failed"
	mappingLabel  = "label"
	scalarValue   = "plain"
	scalarYAML    = scalarValue + "\n"
	validMetaYAML = "" +
		"target_folder: taskfiles\n" +
		"lock_file: taskfiles/.taskotter-lock.yml\n" +
		"configuration_hash: deadbeef\n"
	badMetaField = "target_folder: {}\n"
)

// TestMarshalMetadataRoundTrip verifies metadata encodes and decodes stably.
func TestMarshalMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	meta := sampleMetadata()
	data := MarshalMetadata(&meta)

	var got Metadata

	unmarshalMetaOK(t, string(data), &got)
	assertMetadataEqual(t, &meta, &got)
}

// TestMetadataUnmarshalYAMLSuccess verifies valid metadata YAML decodes.
func TestMetadataUnmarshalYAMLSuccess(t *testing.T) {
	t.Parallel()
	runMetaOKCases(t, metaOKCases())
}

// TestMetadataUnmarshalYAMLFailures verifies invalid metadata YAML fails.
func TestMetadataUnmarshalYAMLFailures(t *testing.T) {
	t.Parallel()
	runMetaFailCases(t, metaFailCases())
}

// TestUnmarshalYAMLMappingRejectsNonMapping verifies scalar roots fail.
func TestUnmarshalYAMLMappingRejectsNonMapping(t *testing.T) {
	t.Parallel()

	err := UnmarshalYAMLMapping(yamlScalarNode(scalarValue), mappingLabel, map[string]any{})
	assertFails(t, err)
}

// TestUnmarshalYAMLMappingRejectsBadField verifies type mismatches fail.
func TestUnmarshalYAMLMappingRejectsBadField(t *testing.T) {
	t.Parallel()

	var folder string

	err := UnmarshalYAMLMapping(
		mustParseYAMLNode(t, badMetaField),
		mappingLabel,
		map[string]any{"target_folder": &folder},
	)
	assertFails(t, err)
}

// TestSyncErrorErrorReturnsMessage verifies SyncError exposes its message.
func TestSyncErrorErrorReturnsMessage(t *testing.T) {
	t.Parallel()

	if SyncError(syncErrText).Error() != syncErrText {
		t.Fatalf("Error() = %q", SyncError(syncErrText).Error())
	}
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertMetadataEqual(t *testing.T, want, got *Metadata) {
	t.Helper()

	if *want != *got {
		t.Fatalf("metadata = %+v, want %+v", got, want)
	}
}

func assertMetaFailCase(t *testing.T, testCase *metaYAMLCase) {
	t.Helper()

	var meta Metadata

	assertFails(t, yaml.Unmarshal([]byte(testCase.payload), &meta))
}

func assertMetaOKCase(t *testing.T, testCase *metaYAMLCase) {
	t.Helper()

	var meta Metadata

	unmarshalMetaOK(t, testCase.payload, &meta)
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

func metaFailCases() []metaYAMLCase {
	return []metaYAMLCase{
		{name: "scalar", payload: scalarYAML},
		{name: "sequence", payload: "- a\n"},
		{name: "bad-field", payload: badMetaField},
	}
}

func metaOKCases() []metaYAMLCase {
	return []metaYAMLCase{
		{name: "valid", payload: validMetaYAML},
		{name: "partial", payload: "lock_file: only\n"},
	}
}

func mustParseDocument(t *testing.T, payload string) *yaml.Node {
	t.Helper()

	var node yaml.Node

	assertNoErr(t, yaml.Unmarshal([]byte(payload), &node))

	return &node
}

func mustParseYAMLNode(t *testing.T, payload string) *yaml.Node {
	t.Helper()

	return mustParseDocument(t, payload).Content[consts.IndexZero]
}

func runMetaFailCase(t *testing.T, testCase *metaYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertMetaFailCase(t, testCase)
	})
}

func runMetaFailCases(t *testing.T, cases []metaYAMLCase) {
	t.Helper()

	for idx := range cases {
		runMetaFailCase(t, &cases[idx])
	}
}

func runMetaOKCase(t *testing.T, testCase *metaYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertMetaOKCase(t, testCase)
	})
}

func runMetaOKCases(t *testing.T, cases []metaYAMLCase) {
	t.Helper()

	for idx := range cases {
		runMetaOKCase(t, &cases[idx])
	}
}

func sampleMetadata() Metadata {
	return Metadata{
		TargetFolder:      "taskfiles",
		LockFile:          "taskfiles/.taskotter-lock.yml",
		ConfigurationHash: "deadbeef",
	}
}

func unmarshalMetaOK(t *testing.T, payload string, meta *Metadata) {
	t.Helper()
	assertNoErr(t, yaml.Unmarshal([]byte(payload), meta))
}

func yamlScalarNode(value string) *yaml.Node {
	//nolint:exhaustruct_v5 // only kind and value matter for this fixture
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value}
}
