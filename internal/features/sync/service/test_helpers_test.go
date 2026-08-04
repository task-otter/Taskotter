// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	synctaskfile "github.com/task-otter/Taskotter/internal/features/sync/adapters/taskfile"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	syncsvc "github.com/task-otter/Taskotter/internal/features/sync/service"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

type (
	copyHookInput struct {
		t    *testing.T
		plan *syncdomain.Plan
		hook func(string, *syncdomain.FileEntry) error
		run  func()
	}

	lockWriteInput struct {
		t            *testing.T
		workspace    string
		targetFolder string
		files        []managed.File
	}

	metadataWriteInput struct {
		t            *testing.T
		workspace    string
		targetFolder string
		meta         []byte
	}

	moduleFileInput struct {
		t       *testing.T
		dir     string
		rel     string
		content string
	}

	preservePathInput struct {
		t         *testing.T
		plan      *syncdomain.Plan
		syncInput *syncdomain.SyncInput
		path      string
	}

	setupPlanArgs struct {
		mutate    func(*config.Config)
		t         *testing.T
		workspace string
	}

	setupPlanRootArgs struct {
		mutate      func(*config.Config)
		t           *testing.T
		workspace   string
		rootContent string
	}

	moduleTestInput struct {
		t    *testing.T
		cfg  *config.Config
		snap *storedomain.Snapshot
	}

	variantModuleSyncArgs struct {
		cfg    *config.Config
		snap   *storedomain.Snapshot
		task   string
		source string
	}
)

const (
	testModuleEslint     = "eslint"
	testModuleGoMetadata = "module: go\n"
	testTaskfileName     = "Taskfile.yml"
	testReadmeName       = "README.md"
	testRelGoSetupSh     = "setup.sh"
	testRelGoObsoleteTxt = "obsolete.txt"

	testLegacyMetaDir          = ".taskotter"
	testGoTaskfilePath         = "go/Taskfile.yml"
	testLockFileName           = ".taskotter-lock.yml"
	testMetadataFileName       = "metadata.yml"
	testFileUserTxt            = "user.txt"
	testStagingDir             = "staging"
	dirTests                   = "tests"
	dirFixtures                = "fixtures"
	dirStore                   = "store"
	notFoundIndex              = -1
	testMetadataRelPath        = ".taskotter/metadata.yml"
	testInvalidYAML            = "{{not yaml"
	testBadYAML                = "{{bad"
	errExpectedCorruptLock     = "expected corrupt lock error"
	errExpectedCorruptMetadata = "expected corrupt metadata error"
	taskGoTaskfilePath         = "task/go/Taskfile.yml"
	contentKeep                = "keep"
	targetFolderTask           = "task"
	docGuideMD                 = "docs/guide.md"
	docNestedNoteMD            = "docs/nested/note.md"
	docsDirName                = "docs"
	fileGoTestGo               = "go_test.go"
	docMetadataYML             = "docs/metadata.yml"
	errExpectedChangesInitial  = "expected changes on initial sync"
	testStoreSourceRef         = "refs/heads/main"
	testStoreResolvedCommit    = "abc123"
	testStoreDefaultBranch     = "main"
	errFmtExpectedManagedPath  = "expected managed path %q"
	testEmptyTaskfileYAML      = "version: \"3\"\ntasks: {}\n"
)

func fatalOnErr(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func mustResolveTask(t *testing.T, input *resolvesvc.ResolveInput) resolvesvc.Resolution {
	t.Helper()

	res, err := resolvesvc.Resolve(input)
	fatalOnErr(t, err)

	return res
}

func resolveGoModule(t *testing.T, snap *storedomain.Snapshot) resolvesvc.Resolution {
	t.Helper()

	return mustResolveTask(t, &resolvesvc.ResolveInput{
		Task: consts.Go, Catalog: snap.Catalog,
		PackageManager: consts.Empty, VersionManager: consts.Empty,
	})
}

func buildSingleSyncIn(input *moduleTestInput) syncdomain.SyncInput {
	input.t.Helper()

	res := resolveGoModule(input.t, input.snap)

	return variantModuleSyncInput(&variantModuleSyncArgs{
		cfg: input.cfg, snap: input.snap, task: consts.Go, source: res.SourceModule,
	})
}

func variantModuleSyncInput(args *variantModuleSyncArgs) syncdomain.SyncInput {
	return syncdomain.SyncInput{
		Config:      args.cfg,
		TaskfileOps: synctaskfile.Ops{},
		Snapshot:    args.snap,
		Requested: map[string]lockmodel.ModuleRecord{
			args.task: {
				SourceModule:      args.source,
				DestinationModule: args.task,
				Path:              config.DefaultTargetFolder + "/" + args.task,
			},
		},
		Dependencies: nil,
		SourceToDest: map[string]string{args.source: args.task},
		DestByTask:   map[string]string{args.task: args.task},
	}
}

func testStoreRefInfo() *storedomain.RefInfo {
	return &storedomain.RefInfo{
		Repository:       config.StoreRepository,
		RequestedVersion: consts.Empty,
		SourceRef:        testStoreSourceRef,
		ResolvedCommit:   testStoreResolvedCommit,
		DefaultBranch:    testStoreDefaultBranch,
	}
}

func resolveAllModules(input *moduleTestInput) []resolvesvc.Resolution {
	input.t.Helper()

	resolutions, err := resolvesvc.ResolveAll(&resolvesvc.ResolveAllInput{
		Tasks: input.cfg.Tasks, Catalog: input.snap.Catalog,
		PackageManager: input.cfg.NodePackageManager, VersionManager: input.cfg.NodeVersionManager,
	})
	fatalOnErr(input.t, err)

	return resolutions
}

func runApplyPlan(t *testing.T, plan *syncdomain.Plan, syncInput *syncdomain.SyncInput) error {
	t.Helper()

	err := syncsvc.ApplyPlan(plan, syncInput)
	if err != nil {
		return fmt.Errorf("apply plan: %w", err)
	}

	return nil
}

func withCopyFileHook(input *copyHookInput) {
	input.t.Helper()

	syncsvc.SetCopyFileToHookForTest(input.plan, input.hook)
	input.t.Cleanup(func() { syncsvc.SetCopyFileToHookForTest(input.plan, nil) })

	input.run()
}

func defaultTestConfig(workspace string) *config.Config {
	return &config.Config{
		Tasks:              nil,
		JSRuntime:          consts.Empty,
		NodePackageManager: consts.Empty,
		NodeVersionManager: consts.Empty,
		IncludesDoc:        false,
		SyncRoot:           true,
		FailOnChanges:      false,
		StoreVersion:       consts.Empty,
		TargetFolder:       config.DefaultTargetFolder,
		RootTaskfile:       testTaskfileName,
		GitHubToken:        consts.Empty,
		Workspace:          workspace,
		Repository:         consts.Empty,
		GitHubOutput:       consts.Empty,
		BaseBranch:         consts.Empty,
		ConfigurationHash:  consts.Empty,
		BranchName:         consts.Empty,
	}
}

func testConfig(workspace string, mutate func(*config.Config)) *config.Config {
	cfg := defaultTestConfig(workspace)

	if mutate != nil {
		mutate(cfg)
	}

	return cfg
}

func writeFileWithDir(t *testing.T, path string, data []byte) {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), consts.FilePerm755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, data, consts.FilePerm644)
	if err != nil {
		t.Fatal(err)
	}
}

func writeTaskotterMetadata(input *metadataWriteInput) {
	input.t.Helper()

	path := filepath.Join(input.workspace, input.targetFolder, testMetadataRelPath)
	writeFileWithDir(input.t, path, input.meta)
}

func writeLockFile(input *lockWriteInput) {
	input.t.Helper()

	var lock lockmodel.LockFile

	lock.Configuration.TargetFolder = input.targetFolder
	lock.ManagedFiles = input.files

	data, err := syncsvc.MarshalLock(&lock)
	if err != nil {
		input.t.Fatal(err)
	}

	lockPath := filepath.Join(input.workspace, input.targetFolder, testLockFileName)
	writeFileWithDir(input.t, lockPath, data)
}

func writeMinimalLock(input *lockWriteInput) {
	input.t.Helper()

	writeLockFile(input)

	meta := []byte(
		"target_folder: " + input.targetFolder +
			"\nlock_file: " + input.targetFolder + "/.taskotter-lock.yml\nconfiguration_hash: x\n",
	)

	writeTaskotterMetadata(&metadataWriteInput{
		t: input.t, workspace: input.workspace, targetFolder: input.targetFolder, meta: meta,
	})
}

func writeModuleFile(input *moduleFileInput) {
	input.t.Helper()

	writeFileWithDir(input.t, filepath.Join(input.dir, input.rel), []byte(input.content))
}

func setupPlan(args *setupPlanArgs) (*config.Config, syncdomain.SyncInput, *syncdomain.Plan) {
	args.t.Helper()

	writeRootTaskfile(args.t, args.workspace)

	cfg := testConfig(args.workspace, args.mutate)
	syncInput, plan := preparePlan(args.t, args.workspace, cfg)

	return cfg, syncInput, plan
}

func replan(t *testing.T, workspace string, cfg *config.Config) *syncdomain.Plan {
	t.Helper()

	syncInput, plan := preparePlan(t, workspace, cfg)
	iox.Discard(syncInput)

	return plan
}

func expectBuildPlanError(t *testing.T, si *syncdomain.SyncInput, wantErr string) {
	t.Helper()

	plan, err := syncsvc.BuildPlan(si)
	iox.Discard(plan)

	if err == nil {
		t.Fatal(wantErr)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	stat, err := os.Stat(path)
	iox.Discard(stat)

	if err != nil {
		t.Fatal(err)
	}
}

func assertApplyPlanPreservesPath(input *preservePathInput) {
	input.t.Helper()

	err := runApplyPlan(input.t, input.plan, input.syncInput)
	if err != nil {
		input.t.Fatal(err)
	}

	assertFileExists(input.t, input.path)
}

func setupPlanInput(args *setupPlanArgs) (syncdomain.SyncInput, *syncdomain.Plan) {
	args.t.Helper()

	cfg, syncInput, plan := setupPlan(args)
	discardCfg(cfg)

	return syncInput, plan
}

func discardCfg(cfg *config.Config) {
	iox.Discard(cfg)
}
