// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rundomain "github.com/task-otter/Taskotter/internal/features/orchestrator/domain"
	"github.com/task-otter/Taskotter/internal/features/orchestrator/reporting"
	"github.com/task-otter/Taskotter/internal/features/orchestrator/service"
	"github.com/task-otter/Taskotter/internal/features/state"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

type (
	versionWarningFixture struct {
		cfg   *config.Config
		deps  *service.Deps
		store *versionStore
	}

	versionStore struct {
		latestErr         error
		localStore        *localStore
		log               *bytes.Buffer
		downloadRef       storedomain.RefInfo
		latest            string
		logBeforeDownload string
		latestCalls       int
	}

	versionWarningCase struct {
		name      string
		requested string
		latest    string
		lookupErr error
		wantLog   string
		wantCalls int
		wantWarn  bool
	}
)

const (
	warningLatestVersion     = "v1.4.0"
	warningVersion19         = "v1.9.0"
	warningVersion110        = "v1.10.0"
	warningPrereleaseVersion = "v1.4.0-rc.1"
)

// TestOrchestratorStoreVersionWarnings verifies advisory checks preserve the pin on changed and unchanged runs.
func TestOrchestratorStoreVersionWarnings(t *testing.T) {
	t.Parallel()

	cases := versionWarningCases()

	for i := range cases {
		testCase := &cases[i]
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assertVersionWarningSync(t, testCase)
		})
	}
}

// TestOrchestratorInvalidPinSkipsVersionCheck verifies tag resolution failures still stop before download.
func TestOrchestratorInvalidPinSkipsVersionCheck(t *testing.T) {
	t.Parallel()

	fixture := versionWarningSetup(t, testStoreVersion)

	fixture.store.localStore.resolveErr = errTestBoom

	result, err := service.NewOrchestrator(fixture.deps).Run(t.Context(), fixture.cfg)
	iox.Discard(result)

	store := fixture.store
	emptyRef := new(refInfo)

	if err == nil || store.latestCalls != consts.IndexZero || store.downloadRef != *emptyRef {
		t.Fatalf("err = %v, latest calls = %d, downloaded ref = %#v",
			err, fixture.store.latestCalls, fixture.store.downloadRef)
	}
}

func (store *versionStore) DownloadSnapshot(ctx context.Context, ref *refInfo) (*snap, error) {
	store.downloadRef = *ref
	store.logBeforeDownload = store.log.String()

	snapshot, err := store.localStore.DownloadSnapshot(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("download version test snapshot: %w", err)
	}

	return snapshot, nil
}

func (store *versionStore) LatestStableVersion(context.Context) (string, error) {
	store.latestCalls++

	return store.latest, store.latestErr
}

func (store *versionStore) ResolveRef(ctx context.Context, requested string) (refInfo, error) {
	ref, err := store.localStore.ResolveRef(ctx, requested)
	if err != nil {
		return refInfo{}, fmt.Errorf("resolve version test ref: %w", err)
	}

	if requested != "" {
		ref.SourceRef = "refs/tags/" + requested
	}

	return ref, nil
}

func assertPinnedState(t *testing.T, cfg *config.Config, result *rundomain.Result) {
	t.Helper()

	lock, err := state.LoadLock(
		cfg.Workspace,
		filepath.Join(cfg.TargetFolder, ".taskotter-lock.yml"),
	)
	if err != nil {
		t.Fatal(err)
	}

	source := lock.Source

	if source != result.Ref || source.RequestedVersion != cfg.StoreVersion {
		t.Fatalf("lock source does not match pinned download: %#v", lock.Source)
	}

	assertPinnedOutputs(t, cfg, result)
}

func assertPinnedOutputs(t *testing.T, cfg *config.Config, result *rundomain.Result) {
	t.Helper()

	data := writeVersionOutputs(t, cfg, result)

	wants := []string{
		"store-version=" + cfg.StoreVersion + consts.Newline,
		"source-ref=" + result.SourceRef + consts.Newline,
		"source-sha=" + result.SourceSHA + consts.Newline,
	}

	for i := range wants {
		if !strings.Contains(data, wants[i]) {
			t.Fatalf("outputs missing %q: %s", wants[i], data)
		}
	}
}

func assertVersionWarningSync(t *testing.T, testCase *versionWarningCase) {
	t.Helper()

	fixture := versionWarningSetup(t, testCase.requested)

	fixture.store.latest = testCase.latest
	fixture.store.latestErr = testCase.lookupErr

	for run := range consts.IndexTwo {
		fixture.assertRun(t, testCase, run)
	}
}

func outdatedVersionWarning(requested, latest string) string {
	return fmt.Sprintf("::warning::Store version %s is outdated; latest stable version is %s. "+
		"Update store-version to %s. Continuing with %s.", requested, latest, latest, requested)
}

func versionWarningSetup(t *testing.T, requested string) *versionWarningFixture {
	t.Helper()

	cfg := testConfig(workspaceWithRootTaskfile(t))

	cfg.StoreVersion = requested
	cfg.GitHubOutput = filepath.Join(t.TempDir(), "outputs")

	store := new(versionStore)

	store.localStore = newLocalStore(t)
	store.log = new(bytes.Buffer)

	deps := versionWarningDeps(store)

	return &versionWarningFixture{cfg: cfg, deps: deps, store: store}
}

func versionWarningDeps(store *versionStore) *service.Deps {
	deps := new(service.Deps)

	deps.StoreClient = store
	deps.Logger = logging.NewWithWriter(store.log)

	return deps
}

func writeVersionOutputs(t *testing.T, cfg *config.Config, result *rundomain.Result) string {
	t.Helper()

	err := reporting.WriteActionOutputs(cfg, result)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cfg.GitHubOutput)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func (fixture *versionWarningFixture) assertRun(
	t *testing.T,
	testCase *versionWarningCase,
	run int,
) {
	t.Helper()

	fixture.store.log.Reset()

	result := runOrchestrator(t, service.NewOrchestrator(fixture.deps), fixture.cfg)

	if result.Changed != (run == consts.IndexZero) {
		t.Fatalf("run %d: Changed = %v", run, result.Changed)
	}

	assertPinnedDownload(t, fixture.store, result)
	fixture.assertVersionLog(t, testCase, run)
	assertPinnedState(t, fixture.cfg, result)
}

func (fixture *versionWarningFixture) assertVersionLog(
	t *testing.T,
	testCase *versionWarningCase,
	run int,
) {
	t.Helper()

	hasWarning := strings.Contains(fixture.store.log.String(), "::warning::")
	logMatches := strings.Contains(fixture.store.logBeforeDownload, testCase.wantLog)

	if hasWarning != testCase.wantWarn || !logMatches {
		t.Fatalf("unexpected logs on run %d: %s", run, fixture.store.log.String())
	}

	if fixture.store.latestCalls != testCase.wantCalls*(run+consts.IndexOne) {
		t.Fatalf(
			"latest calls = %d, want %d per run",
			fixture.store.latestCalls,
			testCase.wantCalls,
		)
	}
}

func assertPinnedDownload(t *testing.T, store *versionStore, result *rundomain.Result) {
	t.Helper()

	ref := store.downloadRef

	if result.Ref != ref || result.StoreVersion != ref.RequestedVersion {
		t.Fatalf("result does not match pinned download: %#v, %#v", result, ref)
	}

	if result.SourceRef != ref.SourceRef || result.SourceSHA != ref.ResolvedCommit {
		t.Fatalf("source outputs do not match pinned download: %#v, %#v", result, ref)
	}
}

//nolint:funlen,maintidx // The table lists every version-check outcome with explicit expected logs.
func versionWarningCases() []versionWarningCase {
	return []versionWarningCase{
		{
			name:      "older",
			requested: testStoreVersion,
			latest:    warningLatestVersion,
			wantCalls: consts.IndexOne,
			wantWarn:  true,
			wantLog:   outdatedVersionWarning(testStoreVersion, warningLatestVersion),
			lookupErr: nil,
		},
		{
			name:      "numeric ordering",
			requested: warningVersion19,
			latest:    warningVersion110,
			wantCalls: consts.IndexOne,
			wantWarn:  true,
			wantLog:   outdatedVersionWarning(warningVersion19, warningVersion110),
			lookupErr: nil,
		},
		{
			name:      "equal",
			requested: warningLatestVersion,
			latest:    warningLatestVersion,
			wantCalls: consts.IndexOne,
			lookupErr: nil,
			wantLog:   "",
			wantWarn:  false,
		},
		{
			name:      "newer",
			requested: "v2.0.0",
			latest:    warningLatestVersion,
			wantCalls: consts.IndexOne,
			lookupErr: nil,
			wantLog:   "",
			wantWarn:  false,
		},
		{
			name:      "equivalent tags",
			requested: "1.4.0+build",
			latest:    warningLatestVersion,
			wantCalls: consts.IndexOne,
			lookupErr: nil,
			wantLog:   "",
			wantWarn:  false,
		},
		{
			name:      "prerelease before stable",
			requested: warningPrereleaseVersion,
			latest:    warningLatestVersion,
			wantCalls: consts.IndexOne,
			wantWarn:  true,
			wantLog:   outdatedVersionWarning(warningPrereleaseVersion, warningLatestVersion),
			lookupErr: nil,
		},
		{
			name:      "newer prerelease",
			requested: "v2.0.0-rc.1",
			latest:    warningLatestVersion,
			wantCalls: consts.IndexOne,
			lookupErr: nil,
			wantLog:   "",
			wantWarn:  false,
		},
		{
			name: "no stable tags", requested: testStoreVersion, wantCalls: consts.IndexOne,
			latest:    "",
			lookupErr: nil,
			wantLog:   "",
			wantWarn:  false,
		},
		{
			name: "unpinned store", latest: warningLatestVersion,
			requested: "",
			lookupErr: nil,
			wantLog:   "",
			wantCalls: consts.IndexZero,
			wantWarn:  false,
		},
		{
			name:      "non-semantic pin",
			requested: "custom",
			latest:    warningLatestVersion,
			wantLog:   "Store version custom is not a semantic version; freshness cannot be compared.",
			lookupErr: nil,
			wantCalls: consts.IndexZero,
			wantWarn:  false,
		},
		{
			name:      "lookup failure",
			requested: testStoreVersion,
			lookupErr: errTestBoom,
			wantCalls: consts.IndexOne,
			wantWarn:  true,
			wantLog:   "::warning::Could not check latest stable store version: test boom. Continuing with v1.2.3.",
			latest:    "",
		},
	}
}
