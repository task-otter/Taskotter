// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package managed

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	yaml "go.yaml.in/yaml/v3"
)

type (
	fileYAMLCase struct {
		name    string
		payload string
	}
)

const (
	wantErrText   = "expected error"
	moduleESLint  = "eslint"
	validFileYAML = "" +
		"source_module: eslint\n" +
		"destination_module: eslint\n" +
		"source_path: Taskfile.yml\n" +
		"path: taskfiles/eslint/Taskfile.yml\n" +
		"sha256: abc\n"
	badMappingField = "source_module: {}\n"
	scalarYAML      = "plain\n"
)

// TestFileUnmarshalYAMLSuccess verifies valid managed file YAML decodes.
func TestFileUnmarshalYAMLSuccess(t *testing.T) {
	t.Parallel()
	runFileOKCases(t, fileOKCases())
}

// TestFileUnmarshalYAMLFailures verifies invalid managed file YAML fails.
func TestFileUnmarshalYAMLFailures(t *testing.T) {
	t.Parallel()
	runFileFailCases(t, fileFailCases())
}

// TestFileUnmarshalYAMLRoundTrip verifies a valid managed file decodes all fields.
func TestFileUnmarshalYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	var file File

	unmarshalFileOK(t, validFileYAML, &file)
	assertValidFile(t, &file)
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertFileFailCase(t *testing.T, testCase *fileYAMLCase) {
	t.Helper()

	var file File

	assertFails(t, yaml.Unmarshal([]byte(testCase.payload), &file))
}

func assertFileOKCase(t *testing.T, testCase *fileYAMLCase) {
	t.Helper()

	var file File

	unmarshalFileOK(t, testCase.payload, &file)
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

func assertValidFile(t *testing.T, file *File) {
	t.Helper()

	if file.SourceModule != moduleESLint || file.SHA256 != "abc" {
		t.Fatalf("file = %+v", file)
	}
}

func fileFailCases() []fileYAMLCase {
	return []fileYAMLCase{
		{name: "scalar", payload: scalarYAML},
		{name: "sequence", payload: "- a\n"},
		{name: "bad-field", payload: badMappingField},
	}
}

func fileOKCases() []fileYAMLCase {
	return []fileYAMLCase{
		{name: "valid", payload: validFileYAML},
		{name: "partial", payload: "path: only\n"},
	}
}

func runFileFailCase(t *testing.T, testCase *fileYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertFileFailCase(t, testCase)
	})
}

func runFileFailCases(t *testing.T, cases []fileYAMLCase) {
	t.Helper()

	for idx := range cases {
		runFileFailCase(t, &cases[idx])
	}
}

func runFileOKCase(t *testing.T, testCase *fileYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertFileOKCase(t, testCase)
	})
}

func runFileOKCases(t *testing.T, cases []fileYAMLCase) {
	t.Helper()

	for idx := range cases {
		runFileOKCase(t, &cases[idx])
	}
}

func unmarshalFileOK(t *testing.T, payload string, file *File) {
	t.Helper()
	assertNoErr(t, yaml.Unmarshal([]byte(payload), file))
}
