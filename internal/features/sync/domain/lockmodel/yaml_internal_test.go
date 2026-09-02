// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package lockmodel

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	yaml "go.yaml.in/yaml/v3"
)

type (
	lockYAMLCase struct {
		name    string
		payload string
	}
)

const (
	wantErrText     = "expected error"
	taskGo          = "go"
	moduleBase      = "base"
	branchMain      = "main"
	caseScalar      = "scalar"
	scalarYAML      = "plain\n"
	emptyLockYAML   = "{}\n"
	lockSectionsHdr = "source: {}\nconfiguration: {}\nresolved_modules: {}\n"
	badSourceNested = "source: []\nconfiguration: {}\nresolved_modules: {}\n"
	badConfigNested = "source: {}\nconfiguration: []\nresolved_modules: {}\n"
	badResolvedNest = "source: {}\nconfiguration: {}\nresolved_modules: []\n"
	badSourceField  = "" +
		"source:\n  repository: {}\n" +
		"configuration: {}\nresolved_modules: {}\n"
	badConfigField = "" +
		"source: {}\nconfiguration:\n  includes_doc: {}\n" +
		"resolved_modules: {}\n"
	badConfigTasks = "" +
		"source: {}\nconfiguration:\n  tasks: {}\n" +
		"resolved_modules: {}\n"
	badRequested = "" +
		"source: {}\nconfiguration: {}\n" +
		"resolved_modules:\n  requested: []\n"
	badDependency = "" +
		"source: {}\nconfiguration: {}\n" +
		"resolved_modules:\n  dependencies:\n    - source_module: {}\n"
	badGenerated = lockSectionsHdr + "generated_root_tasks: {}\n"
	badManaged   = lockSectionsHdr + "managed_files: x\n"
	validRecord  = "" +
		"source_module: go\n" +
		"destination_module: go\n" +
		"path: taskfiles/go\n"
	badRecordField = "source_module: {}\n"
)

// TestMarshalLockRoundTrip verifies a populated lock encodes and decodes.
func TestMarshalLockRoundTrip(t *testing.T) {
	t.Parallel()

	lock := sampleLock()
	roundTripLock(t, &lock)
}

// TestMarshalLockEmptySections verifies empty locks omit optional encode branches.
func TestMarshalLockEmptySections(t *testing.T) {
	t.Parallel()

	lock := emptyLock()
	roundTripLock(t, &lock)
}

// TestLockFileUnmarshalYAMLSuccess verifies valid lock YAML decodes.
func TestLockFileUnmarshalYAMLSuccess(t *testing.T) {
	t.Parallel()
	runLockOKCases(t, lockOKCases())
}

// TestLockFileUnmarshalYAMLFailures verifies invalid lock YAML fails.
func TestLockFileUnmarshalYAMLFailures(t *testing.T) {
	t.Parallel()
	runLockFailCases(t, lockFailCases())
}

// TestModuleRecordUnmarshalYAMLSuccess verifies valid module record YAML decodes.
func TestModuleRecordUnmarshalYAMLSuccess(t *testing.T) {
	t.Parallel()
	runRecordOKCases(t, recordOKCases())
}

// TestModuleRecordUnmarshalYAMLFailures verifies invalid module record YAML fails.
func TestModuleRecordUnmarshalYAMLFailures(t *testing.T) {
	t.Parallel()
	runRecordFailCases(t, recordFailCases())
}

// TestOrderedRequestedUnmarshalRejectsSequence verifies non-mapping requested fails.
func TestOrderedRequestedUnmarshalRejectsSequence(t *testing.T) {
	t.Parallel()

	var requested OrderedRequested

	assertFails(t, yaml.Unmarshal([]byte("- a\n"), &requested))
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertLockFailCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	var lock LockFile

	assertFails(t, yaml.Unmarshal([]byte(testCase.payload), &lock))
}

func assertLockOKCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	var lock LockFile

	assertNoErr(t, yaml.Unmarshal([]byte(testCase.payload), &lock))
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}
}

func assertRecordFailCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	var record ModuleRecord

	assertFails(t, yaml.Unmarshal([]byte(testCase.payload), &record))
}

func assertRecordOKCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	var record ModuleRecord

	assertNoErr(t, yaml.Unmarshal([]byte(testCase.payload), &record))
}

func emptyLock() LockFile {
	return LockFile{
		Source:             emptySource(),
		Requested:          OrderedRequested{},
		Dependencies:       nil,
		GeneratedRootTasks: nil,
		ManagedFiles:       nil,
		Configuration:      emptyConfig(),
	}
}

func emptyConfig() LockConfiguration {
	return LockConfiguration{
		TargetFolder:       consts.Empty,
		NodePackageManager: consts.Empty,
		Tasks:              nil,
		IncludesDoc:        false,
		SyncRoot:           false,
	}
}

func emptySource() LockSource {
	return LockSource{
		Repository:       consts.Empty,
		RequestedVersion: consts.Empty,
		SourceRef:        consts.Empty,
		ResolvedCommit:   consts.Empty,
		DefaultBranch:    consts.Empty,
	}
}

func lockFailCases() []lockYAMLCase {
	return []lockYAMLCase{
		{name: caseScalar, payload: scalarYAML},
		{name: "bad-source", payload: badSourceNested},
		{name: "bad-config", payload: badConfigNested},
		{name: "bad-resolved", payload: badResolvedNest},
		{name: "bad-source-field", payload: badSourceField},
		{name: "bad-config-field", payload: badConfigField},
		{name: "bad-config-tasks", payload: badConfigTasks},
		{name: "bad-requested", payload: badRequested},
		{name: "bad-dependency", payload: badDependency},
		{name: "bad-generated", payload: badGenerated},
		{name: "bad-managed", payload: badManaged},
	}
}

func lockOKCases() []lockYAMLCase {
	return []lockYAMLCase{
		{name: "empty", payload: emptyLockYAML},
	}
}

func recordFailCases() []lockYAMLCase {
	return []lockYAMLCase{
		{name: caseScalar, payload: scalarYAML},
		{name: "bad-field", payload: badRecordField},
	}
}

func recordOKCases() []lockYAMLCase {
	return []lockYAMLCase{
		{name: "valid", payload: validRecord},
	}
}

func roundTripLock(t *testing.T, lock *LockFile) {
	t.Helper()

	var got LockFile

	assertNoErr(t, yaml.Unmarshal(MarshalLock(lock), &got))
}

func runLockFailCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertLockFailCase(t, testCase)
	})
}

func runLockFailCases(t *testing.T, cases []lockYAMLCase) {
	t.Helper()

	for idx := range cases {
		runLockFailCase(t, &cases[idx])
	}
}

func runLockOKCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertLockOKCase(t, testCase)
	})
}

func runLockOKCases(t *testing.T, cases []lockYAMLCase) {
	t.Helper()

	for idx := range cases {
		runLockOKCase(t, &cases[idx])
	}
}

func runRecordFailCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertRecordFailCase(t, testCase)
	})
}

func runRecordFailCases(t *testing.T, cases []lockYAMLCase) {
	t.Helper()

	for idx := range cases {
		runRecordFailCase(t, &cases[idx])
	}
}

func runRecordOKCase(t *testing.T, testCase *lockYAMLCase) {
	t.Helper()

	t.Run(testCase.name, func(t *testing.T) {
		t.Parallel()
		assertRecordOKCase(t, testCase)
	})
}

func runRecordOKCases(t *testing.T, cases []lockYAMLCase) {
	t.Helper()

	for idx := range cases {
		runRecordOKCase(t, &cases[idx])
	}
}

func sampleConfig() LockConfiguration {
	return LockConfiguration{
		TargetFolder:       "taskfiles",
		NodePackageManager: "pnpm",
		Tasks:              []string{taskGo},
		IncludesDoc:        true,
		SyncRoot:           true,
	}
}

func sampleDeps() []ModuleRecord {
	return []ModuleRecord{{
		SourceModule:      moduleBase,
		DestinationModule: moduleBase,
		Path:              consts.Empty,
	}}
}

func sampleLock() LockFile {
	return LockFile{
		Source:             sampleSource(),
		Requested:          sampleRequested(),
		Dependencies:       sampleDeps(),
		GeneratedRootTasks: []string{"ci"},
		ManagedFiles:       sampleManaged(),
		Configuration:      sampleConfig(),
	}
}

func sampleManaged() []managed.File {
	return []managed.File{{
		SourceModule:      taskGo,
		DestinationModule: taskGo,
		SourcePath:        consts.Taskfile,
		Path:              "taskfiles/go/Taskfile.yml",
		SHA256:            "deadbeef",
	}}
}

func sampleRequested() OrderedRequested {
	return OrderedRequested{
		taskGo: {
			SourceModule:      taskGo,
			DestinationModule: taskGo,
			Path:              "taskfiles/go",
		},
	}
}

func sampleSource() LockSource {
	return LockSource{
		Repository:       "task-otter/store",
		RequestedVersion: branchMain,
		SourceRef:        "refs/heads/main",
		ResolvedCommit:   "abc",
		DefaultBranch:    branchMain,
	}
}
