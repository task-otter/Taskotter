// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"errors"
	"testing"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/managed"
	"github.com/task-otter/Taskotter/internal/shared/config"
)

type (
	assertRootSkippedInput struct {
		t           *testing.T
		workspace   string
		rootPath    string
		rootContent string
	}

	writeOrderInput struct {
		t         *testing.T
		plan      *syncdomain.Plan
		syncInput *syncdomain.SyncInput
		workspace string
	}

	copyRecordInput struct {
		order     *[]string
		workspace string
		marker    string
	}

	buildPlanFromInput struct {
		t           *testing.T
		cfg         *config.Config
		snap        *storedomain.Snapshot
		resolutions []resolvesvc.Resolution
		depSources  []string
	}

	migrationAssertInput struct {
		t          *testing.T
		workspace  string
		oldManaged string
		oldUser    string
	}

	oldTargetFixture struct {
		oldManaged string
		oldUser    string
	}

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
	parentDocTool       = "tool"
	parentDocNode       = "tool/node"
	parentDocLeaf       = "tool/node/pnpm"
	parentDocRootReadme = "root-readme\n"
	parentDocLeafReadme = "leaf-readme\n"

	currentSchemaMetadata = "---\nschema: taskotter.dev/taskfile-metadata/v1\n" +
		"module: tool/node/pnpm\ntaskfile: Taskfile.yml\nexported_tasks: [install]\n"
	legacySchemaMetadata = "---\nschema: taskotter.store/v1\n" +
		"module: tool/node/pnpm\ntaskfile: Taskfile.yml\nexported_tasks: [install]\n"

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

var errSimulatedPromoteFailure = errors.New("simulated promote failure")
