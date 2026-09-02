// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	yaml "go.yaml.in/yaml/v3"
)

type (
	tempEntry struct {
		entry os.DirEntry
		root  string
	}
)

const (
	wantErrText      = "expected error"
	unexpectFmt      = "unexpected error: %v"
	stagingName      = "staging-root"
	fileNameTxt      = "file.txt"
	payloadText      = "payload"
	badYAMLText      = "{{bad"
	byteX            = "x"
	pathA            = "a"
	pathB            = "b"
	outName          = "o"
	srcDir           = "/src"
	srcFileA         = "/src/a"
	wsRoot           = "/ws"
	wsFileX          = "/ws/x"
	rootDir          = "/root"
	rootMetaPath     = "/root/go/metadata.yml"
	goTaskfileRel    = "taskfiles/go/Taskfile.yml"
	goOldTxtRel      = "taskfiles/go/old.txt"
	oldTargetFolder  = "old-taskfiles"
	oldTargetFileRel = "old-taskfiles/go/a.txt"
	otherMetaRel     = "other/.taskotter/metadata.yml"
	missingTxt       = "missing.txt"
	gitDirName       = ".git"
	errWantFmt       = "err = %v, want %v"
	errSkipDirFmt    = "err = %v, want SkipDir"
	listsFmt         = "lists = %+v"
	relScanFmt       = "rel=%q scan=%v"
	errBareFmt       = "err = %v"
	removeEmptyCtx   = "remove empty"
	eslintNodePNPM   = "eslint/node/pnpm"
	eslintBun        = "eslint/bun"
	taskCI           = "ci"
	pathMissing      = "missing"

	unknownFileChange = 99

	subDirName  = "sub"
	goSubOldRel = "taskfiles/go/sub/old.txt"
	gotFmt      = "got = %v"
	modXName    = "mod-x"
)

var errStub = errors.New("stub failure")

func emptyLock() syncLock {
	var lock syncLock

	return lock
}

func emptyLockPtr() *syncLock {
	lock := emptyLock()

	return &lock
}

func emptyMetadata(lockFile string) domain.Metadata {
	var meta domain.Metadata

	meta.LockFile = lockFile

	return meta
}

func emptyMetadataPtr(lockFile string) *domain.Metadata {
	meta := emptyMetadata(lockFile)

	return &meta
}

func emptyModule(source, dest string) moduleRecord {
	var mod moduleRecord

	mod.SourceModule = source
	mod.DestinationModule = dest

	return mod
}

func emptyModulePtr(source, dest string) *moduleRecord {
	mod := emptyModule(source, dest)

	return &mod
}

func managedAt(path, dest, sha string) managedFile {
	var file managedFile

	file.Path = path
	file.DestinationModule = dest
	file.SHA256 = sha

	return file
}

func managedAtPtr(path, dest, sha string) *managedFile {
	file := managedAt(path, dest, sha)

	return &file
}

func emptyConfig() config.Config {
	var cfg config.Config

	return cfg
}

func emptyConfigPtr() *config.Config {
	cfg := emptyConfig()

	return &cfg
}

func emptySyncInput() domain.SyncInput {
	var input domain.SyncInput

	return input
}

func emptySyncInputPtr() *domain.SyncInput {
	input := emptySyncInput()

	return &input
}

func emptyPlan() domain.Plan {
	var plan domain.Plan

	return plan
}

func planWithLockMeta(lockFile string) *domain.Plan {
	plan := emptyPlan()

	plan.Metadata = emptyMetadata(lockFile)

	return &plan
}

func syncInputWithConfig() *domain.SyncInput {
	syncIn := emptySyncInput()
	cfg := emptyConfig()

	syncIn.Config = &cfg

	return &syncIn
}

func lockWithManaged(path string) *syncLock {
	lock := emptyLock()

	lock.ManagedFiles = []managedFile{managedAt(path, consts.Go, consts.Empty)}

	return &lock
}

func newStagingSession(copyFile func(string, *domain.FileEntry) error) stagingSession {
	var session stagingSession

	session.copyFile = copyFile

	return session
}

func scalarYAMLNode(value string) *yaml.Node {
	var node yaml.Node

	node.Kind = yaml.ScalarNode
	node.Value = value

	return &node
}

func zeroDiffLists() *diffLists {
	var lists diffLists

	return &lists
}

func newTempEntry(t *testing.T) tempEntry {
	t.Helper()

	root := t.TempDir()
	writeTempFile(t, filepath.Join(root, fileNameTxt), []byte(byteX))

	entries, err := os.ReadDir(root)
	assertNoErr(t, err)

	return tempEntry{entry: entries[consts.IndexZero], root: root}
}

func missingFileEntryArgs(t *testing.T) *fileEntryArgs {
	t.Helper()

	temp := newTempEntry(t)

	var args fileEntryArgs

	args.entry = temp.entry
	args.sourceDir = temp.root
	args.rel = missingTxt
	args.absPath = missingTxt

	return &args
}

func goModulePlanArgs(workspace string) *modulePlanArgs {
	var args modulePlanArgs

	var syncIn domain.SyncInput

	syncIn.Config = emptyConfigPtr()
	syncIn.Snapshot = dirSnapshot{root: workspace}

	args.syncInput = &syncIn
	args.mod = emptyModulePtr(consts.Go, consts.Go)
	args.moduleContents = mcMap{}

	return &args
}

func goTaskMetadataInput(want *storeTaskMetadata) *groupModulesInput {
	var input groupModulesInput

	input.requestedRecords = map[string]moduleRecord{
		consts.Go: emptyModule(consts.Go, consts.Empty),
	}
	input.metadata = storeTaskMetaMap{consts.Go: *want}

	return &input
}

func sortedManagedFixture() []managedFile {
	planned := []managedFile{
		managedAt(pathB, consts.Empty, consts.Empty),
		managedAt(pathA, consts.Empty, consts.Empty),
		managedAt(pathA, consts.Empty, consts.Empty),
	}

	planned[consts.IndexZero].SourceModule = "mod-z"
	planned[consts.IndexOne].SourceModule = "mod-y"
	planned[consts.IndexTwo].SourceModule = modXName

	return planned
}

func prepFinalizeDiffWorkspace(t *testing.T) string {
	t.Helper()

	workspace := t.TempDir()
	assertNoErr(
		t,
		mkdirAll(filepath.Join(workspace, filepath.FromSlash(goTaskfileRel)), dirModePerm),
	)

	return workspace
}

func finalizeDiffFailInput(workspace string) *finalizeBuiltPlanInput {
	plan := emptyPlan()

	plan.ManagedFiles = []managedFile{managedAt(goTaskfileRel, consts.Empty, byteX)}
	plan.Metadata = emptyMetadata(lockRelPath)

	var artifacts planArtifacts

	return &finalizeBuiltPlanInput{
		syncInput: minimalSyncInput(workspace),
		plan:      &plan,
		meta:      emptyMetadataPtr(lockRelPath),
		artifacts: &artifacts,
	}
}

func assertCandidateRelFails(
	t *testing.T,
	call func(*metadataCandidateArgs) (string, metadataScanResult, error),
) {
	t.Helper()

	swapRelPath(t, failingRelPath)

	rel, scan, err := call(&metadataCandidateArgs{
		workspace:           wsRoot,
		currentMetadataPath: metaRelPath,
		abs:                 wsFileX,
		entry:               fakeDirEntry{name: fileNameTxt, dir: false},
	})

	iox.Discard(rel)
	iox.Discard(scan)
	assertFails(t, err)
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf(unexpectFmt, err)
	}
}

func failingChmod(*os.File, os.FileMode) error { return errStub }
func failingClose(*os.File) error              { return errStub }
func failingCreateTemp(string, string) (*os.File, error) {
	return nil, errStub
}

func failingMarshalYAML(any) ([]byte, error)    { return nil, errStub }
func failingMkdirAll(string, os.FileMode) error { return errStub }

func failingMkdirTemp(string, string) (string, error) {
	return consts.Empty, errStub
}

func failingOpenRelative(string, string) (*os.File, error) {
	return nil, errStub
}
func failingReadAll(io.Reader) ([]byte, error) { return nil, errStub }

func failingRelPath(string, string) (string, error) {
	return consts.Empty, errStub
}
func failingRemove(string) error { return errStub }

func failingRename(string, string) error { return errStub }

//nolint:ireturn // interface required by stdlib signature
func failingStat(string) (os.FileInfo, error) { return nil, errStub }

func failingWalk(string, fs.WalkDirFunc) error { return errStub }

func failingWriteFull(io.Writer, []byte) error { return errStub }

func prepGoSubDir(t *testing.T) string {
	t.Helper()

	workspace := t.TempDir()
	nested := filepath.Join(workspace, taskfilesDirName, consts.Go, subDirName)
	assertNoErr(t, mkdirAll(nested, dirModePerm))

	return workspace
}

func succeedThenFail() func(string) error {
	calls := consts.IndexZero

	return func(string) error {
		calls++

		if calls == consts.IndexOne {
			return nil
		}

		return errStub
	}
}

func swapSeam[T any](t *testing.T, target *T, stub T) {
	t.Helper()

	original := *target

	*target = stub

	t.Cleanup(func() { *target = original })
}

func swapChmodFile(t *testing.T, stub func(*os.File, os.FileMode) error) {
	t.Helper()

	swapSeam(t, &chmodFile, stub)
}

func swapCloseFile(t *testing.T, stub func(*os.File) error) {
	t.Helper()

	swapSeam(t, &closeFile, stub)
}

func swapCreateTemp(t *testing.T, stub func(string, string) (*os.File, error)) {
	t.Helper()

	swapSeam(t, &createTemp, stub)
}

func swapMarshalYAML(t *testing.T, stub func(any) ([]byte, error)) {
	t.Helper()

	swapSeam(t, &marshalYAML, stub)
}

func swapMkdirAll(t *testing.T, stub func(string, os.FileMode) error) {
	t.Helper()

	swapSeam(t, &mkdirAll, stub)
}

func swapMkdirTemp(t *testing.T, stub func(string, string) (string, error)) {
	t.Helper()

	swapSeam(t, &mkdirTemp, stub)
}

func swapOpenRelative(t *testing.T, stub func(string, string) (*os.File, error)) {
	t.Helper()

	swapSeam(t, &openRelativeFile, stub)
}

func swapReadAll(t *testing.T, stub func(io.Reader) ([]byte, error)) {
	t.Helper()

	swapSeam(t, &readAll, stub)
}

func swapRelPath(t *testing.T, stub func(string, string) (string, error)) {
	t.Helper()

	swapSeam(t, &relPath, stub)
}

func swapRemoveAll(t *testing.T, stub func(string) error) {
	t.Helper()

	swapSeam(t, &removeAll, stub)
}

func swapRemovePath(t *testing.T, stub func(string) error) {
	t.Helper()

	swapSeam(t, &removePath, stub)
}

func swapRenamePath(t *testing.T, stub func(string, string) error) {
	t.Helper()

	swapSeam(t, &renamePath, stub)
}

func swapStatPath(t *testing.T, stub func(string) (os.FileInfo, error)) {
	t.Helper()

	swapSeam(t, &statPath, stub)
}

func swapWalkDir(t *testing.T, stub func(string, fs.WalkDirFunc) error) {
	t.Helper()

	swapSeam(t, &walkDir, stub)
}

func swapWriteFull(t *testing.T, stub func(io.Writer, []byte) error) {
	t.Helper()

	swapSeam(t, &writeFull, stub)
}
