// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

import (
	"testing"

	yaml "go.yaml.in/yaml/v3"
)

const (
	testLabel      = "test"
	testNumber     = "number"
	testNotANumber = "not-a-number"
	testMissing    = "missing"
	testMessage    = "message"
	testKnown      = "known"
)

// TestCodecErrorPaths verifies codec entry points return errors for invalid input.
func TestCodecErrorPaths(t *testing.T) {
	t.Parallel()
	assertInvalidDocumentErrors(t)
	assertInvalidMappingErrors(t)
}

func assertInvalidDocumentErrors(t *testing.T) {
	t.Helper()

	assertCodecError(t, DecodeMetadataYAML([]byte(testBadYAML), &Metadata{}))
	assertCodecError(t, DecodeMetadataYAML(nil, &Metadata{}))
	assertDecodeMetadataFailure(t)
	assertDecodeLockFailure(t)
	assertCodecError(t, UnmarshalYAMLMapping(&yaml.Node{Kind: yaml.ScalarNode}, testLabel, nil))
	assertCodecError(t, UnmarshalMetadata(&yaml.Node{Kind: yaml.ScalarNode}, &Metadata{}))
}

func assertDecodeMetadataFailure(t *testing.T) {
	t.Helper()

	metadata, err := DecodeMetadata([]byte(testBadYAML))
	assertCodecError(t, err)

	if metadata != nil {
		t.Fatal("DecodeMetadata() returned metadata with invalid YAML")
	}
}

func assertDecodeLockFailure(t *testing.T) {
	t.Helper()

	lock, err := DecodeLock([]byte(testBadYAML))
	assertCodecError(t, err)

	if lock != nil {
		t.Fatal("DecodeLock() returned a lock with invalid YAML")
	}
}

func assertInvalidMappingErrors(t *testing.T) {
	t.Helper()

	assertCodecError(t, decodeYAMLField(map[string]*yaml.Node{
		testNumber: {Kind: yaml.ScalarNode, Value: testNotANumber},
	}, testNumber, new(int)))
	assertCodecError(t, decodeYAMLFields(map[string]*yaml.Node{
		testNumber: {Kind: yaml.ScalarNode, Value: testNotANumber},
	}, struct {
		Out any
		Key string
	}{Key: testNumber, Out: new(int)}))
	assertCodecError(t, unmarshalYAMLTargets(
		mappingNode(t, testNumber+": "+testNotANumber+"\n"),
		testLabel,
		struct {
			Out any
			Key string
		}{Key: testNumber, Out: new(int)},
	))
}

// TestCodecHelpersHandleDocumentsAndMissingFields verifies document and optional-field behavior.
func TestCodecHelpersHandleDocumentsAndMissingFields(t *testing.T) {
	t.Parallel()
	assertDocumentContent(t)
	assertMappingFieldHandling(t)
	assertMissingField(t)
}

func assertDocumentContent(t *testing.T) {
	t.Helper()

	if yamlDocumentContent(nil) != nil {
		t.Fatal("yamlDocumentContent(nil) did not return nil")
	}

	emptyDocument := &yaml.Node{Kind: yaml.DocumentNode}

	if yamlDocumentContent(emptyDocument) != emptyDocument {
		t.Fatal("empty document was not preserved")
	}
}

func assertMappingFieldHandling(t *testing.T) {
	t.Helper()

	var target string

	err := UnmarshalYAMLMapping(
		mappingNode(t, "known: value\n"),
		testLabel,
		map[string]any{testKnown: &target, testMissing: new(string)},
	)
	if err != nil {
		t.Fatalf("UnmarshalYAMLMapping() error = %v", err)
	}

	if target != "value" {
		t.Fatalf("decoded value = %q, want value", target)
	}
}

func assertMissingField(t *testing.T) {
	t.Helper()

	err := decodeYAMLField(nil, testMissing, new(string))
	if err != nil {
		t.Fatalf("decodeYAMLField() error = %v", err)
	}
}

// TestSyncErrorReturnsMessage verifies the error string is preserved.
func TestSyncErrorReturnsMessage(t *testing.T) {
	t.Parallel()

	if got := SyncError(testMessage).Error(); got != testMessage {
		t.Fatalf("Error() = %q, want message", got)
	}
}

func assertCodecError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected codec error")
	}
}

func mappingNode(t *testing.T, source string) *yaml.Node {
	t.Helper()

	var document yaml.Node

	err := yaml.Unmarshal([]byte(source), &document)
	if err != nil {
		t.Fatal(err)
	}

	return yamlDocumentContent(&document)
}
