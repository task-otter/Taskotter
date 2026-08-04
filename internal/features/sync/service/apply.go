// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package service plans and applies taskfile synchronization changes to the workspace.
package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
)

type (
	stagedFile struct {
		finalRel string
		entry    domain.FileEntry
	}

	removeStaleFileArgs struct {
		old          *managedFile
		current      map[string]struct{}
		workspace    string
		targetFolder string
	}
)

const (
	lockFileName = ".taskotter-lock.yml"

	legacyMetadataDirName = ".taskotter"

	legacyMetadataRelPath = ".taskotter/metadata.yml"

	errCreateStagingDir = "create staging directory: %w"

	errValidateRootTaskfile = "validate root Taskfile.yml: %w"

	errValidateLockFile = "validate lock file: %w"

	errValidateMetadata = "validate metadata: %w"

	errFmtCleanupStagingDir = "clean up staging directory %q: %w"

	errFmtRemoveObsoleteFile = "remove obsolete file %q: %w"

	errFmtRemoveStaleManaged = "remove stale managed file: %w"
)

// ApplyPlan writes planned files atomically and removes obsolete managed paths.
func ApplyPlan(plan *domain.Plan, syncInput *domain.SyncInput) error {
	err := applyPlanWithCleanup(plan, syncInput)
	if err != nil {
		return fmt.Errorf("apply plan: %w", err)
	}

	return nil
}

func applyPlanWithCleanup(plan *domain.Plan, syncInput *domain.SyncInput) (err error) {
	session, err := startApplySession(plan, syncInput)
	if err != nil {
		return fmt.Errorf("start apply session: %w", err)
	}

	defer cleanupStagingOnExit(session.stagingRoot, &err)

	err = executeAppliedPlan(&session, plan, syncInput)
	if err != nil {
		return fmt.Errorf("execute applied plan: %w", err)
	}

	return nil
}

//nolint:gocritic // named result requires error pointer for defer cleanup
func cleanupStagingOnExit(stagingRoot string, err *error) {
	cleanupErr := cleanupStagingDir(stagingRoot)

	if cleanupErr != nil && *err == nil {
		*err = cleanupErr
	}
}

func executeAppliedPlan(
	session *stagingSession,
	plan *domain.Plan,
	syncInput *domain.SyncInput,
) error {
	err := runAppliedStagedPlan(&applyStagedInput{
		plan:      plan,
		syncInput: syncInput,
		workspace: syncInput.Config.Workspace,
		session:   *session,
	})
	if err != nil {
		return fmt.Errorf("run applied staged plan: %w", err)
	}

	return nil
}

func startApplySession(plan *domain.Plan, syncInput *domain.SyncInput) (stagingSession, error) {
	session, err := prepareStaging(&prepareStagingInput{
		plan:      plan,
		syncInput: syncInput,
		workspace: syncInput.Config.Workspace,
	})
	if err != nil {
		return stagingSession{}, fmt.Errorf("prepare staging: %w", err)
	}

	return session, nil
}

func runAppliedStagedPlan(input *applyStagedInput) error {
	err := applyStagedPlan(input)
	if err != nil {
		return fmt.Errorf("apply staged plan: %w", err)
	}

	return nil
}

func applyStagedPlan(input *applyStagedInput) error {
	err := validateAndWriteStaged(&validateWriteStagedInput{
		args: validateStagedArgs{
			staged:   input.session.staged,
			rootPath: input.plan.RootTaskfilePath,
		},
		workspace: input.workspace,
		copyFile:  input.session.copyFile,
	})
	if err != nil {
		return fmt.Errorf("validate and write staged files: %w", err)
	}

	err = cleanupAfterApply(input.plan, input.workspace, input.syncInput.Config.MetadataPath())
	if err != nil {
		return fmt.Errorf("clean up after apply: %w", err)
	}

	return nil
}

func buildCurrentPathSet(plan *domain.Plan) map[string]struct{} {
	currentPaths := make(map[string]struct{}, len(plan.ManagedFiles))

	for i := range plan.ManagedFiles {
		currentPaths[plan.ManagedFiles[i].Path] = struct{}{}
	}

	return currentPaths
}

func buildStagedFiles(plan *domain.Plan, syncInput *domain.SyncInput) ([]stagedFile, error) {
	staged := stageModuleFiles(plan, syncInput.Config.TargetFolder)

	if syncInput.Config.SyncRoot {
		staged = append(staged, stagedFile{
			finalRel: plan.RootTaskfilePath,
			entry:    domain.FileEntry{Data: plan.RootTaskfile, Mode: fileModeRegular},
		})
	}

	lockAndMeta, err := stageLockAndMetadata(plan, syncInput.Config.MetadataPath())
	if err != nil {
		return nil, fmt.Errorf("stage lock and metadata: %w", err)
	}

	return append(staged, lockAndMeta...), nil
}

func cleanupAfterApply(plan *domain.Plan, workspace, metadataPath string) error {
	err := removeObsolete(plan, workspace)
	if err != nil {
		return fmt.Errorf("remove obsolete files: %w", err)
	}

	err = cleanupLegacyMetadata(workspace, metadataPath)
	if err != nil {
		return fmt.Errorf("clean up legacy metadata: %w", err)
	}

	return nil
}

func cleanupFailedStaging(stagingRoot string, copyErr error) error {
	removeErr := os.RemoveAll(stagingRoot)
	if removeErr != nil {
		return errors.Join(
			copyErr,
			fmt.Errorf(errFmtCleanupStagingDir, stagingRoot, removeErr),
		)
	}

	return copyErr
}

func cleanupLegacyMetadata(workspace, metadataPath string) error {
	if metadataPath == config.LegacyMetadataPath {
		return nil
	}

	err := removeLegacyMetadataFile(workspace)
	if err != nil {
		return fmt.Errorf("remove legacy metadata file: %w", err)
	}

	err = removeLegacyMetadataDir(workspace)
	if err != nil {
		return fmt.Errorf("remove legacy metadata directory: %w", err)
	}

	return nil
}

func cleanupOldTarget(plan *domain.Plan, workspace string) error {
	if oldTargetUnchanged(plan) {
		return nil
	}

	err := runOldTargetCleanupSteps(plan, workspace)
	if err != nil {
		return fmt.Errorf("run old target cleanup steps: %w", err)
	}

	return nil
}

func runOldTargetCleanupSteps(plan *domain.Plan, workspace string) error {
	steps := []func(*domain.Plan, string) error{
		removeOldTargetFiles,
		removeOldTargetLock,
		removeOldTargetMetadata,
	}

	for i := range steps {
		err := steps[i](plan, workspace)
		if err != nil {
			return fmt.Errorf("old target cleanup: %w", err)
		}
	}

	return nil
}

func cleanupStagingDir(stagingRoot string) error {
	removeErr := os.RemoveAll(stagingRoot)
	if removeErr != nil {
		return fmt.Errorf(errFmtCleanupStagingDir, stagingRoot, removeErr)
	}

	return nil
}

func copyStagedFiles(
	root string,
	staged []stagedFile,
	copyFn func(string, *domain.FileEntry) error,
) error {
	for i := range staged {
		stagedEntry := &staged[i]
		stagePath := filepath.Join(root, filepath.FromSlash(stagedEntry.finalRel))

		err := copyFn(stagePath, &stagedEntry.entry)
		if err != nil {
			return fmt.Errorf("stage %q: %w", stagedEntry.finalRel, err)
		}
	}

	return nil
}

func errorsIsDirectoryNotEmpty(err error) bool {
	return err != nil && (errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST))
}

func oldTargetUnchanged(plan *domain.Plan) bool {
	return plan.OldLock == nil || plan.OldTargetFolder == "" ||
		plan.OldTargetFolder == plan.Metadata.TargetFolder
}

func prepareStaging(input *prepareStagingInput) (stagingSession, error) {
	staged, err := buildStagedFiles(input.plan, input.syncInput)
	if err != nil {
		return stagingSession{}, fmt.Errorf("build staged files: %w", err)
	}

	session, err := stagePreparedFiles(&stagePreparedInput{
		staged:    staged,
		plan:      input.plan,
		syncInput: input.syncInput,
		workspace: input.workspace,
	})
	if err != nil {
		return stagingSession{}, fmt.Errorf("stage prepared files: %w", err)
	}

	return session, nil
}

func prepareStagingRoot(workspace, targetFolder string) (string, error) {
	stagingParent := pathutil.WorkspacePath(
		workspace,
		pathutil.JoinRelative(targetFolder, ".taskotter/staging"),
	)

	err := os.MkdirAll(stagingParent, dirModePerm)
	if err != nil {
		return consts.Empty, fmt.Errorf(errCreateStagingDir, err)
	}

	stagingRoot, err := os.MkdirTemp(stagingParent, "apply-*")
	if err != nil {
		return consts.Empty, fmt.Errorf(errCreateStagingDir, err)
	}

	return stagingRoot, nil
}

func removeDirIfEmpty(dir, context string) error {
	err := os.Remove(dir)

	if err != nil && !os.IsNotExist(err) && !errorsIsDirectoryNotEmpty(err) {
		return fmt.Errorf("%s: %w", context, err)
	}

	return nil
}

func removeIfExists(workspace, rel string) error {
	err := os.Remove(pathutil.WorkspacePath(workspace, rel))

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %q: %w", rel, err)
	}

	return nil
}

func removeLegacyMetadataDir(workspace string) error {
	legacyDir := pathutil.WorkspacePath(workspace, legacyMetadataDirName)

	err := removeDirIfEmpty(legacyDir, "remove legacy metadata directory")
	if err != nil {
		return fmt.Errorf("clean up legacy metadata directory: %w", err)
	}

	return nil
}

func removeLegacyMetadataFile(workspace string) error {
	legacy := pathutil.WorkspacePath(workspace, config.LegacyMetadataPath)

	err := os.Remove(legacy)

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy metadata: %w", err)
	}

	return nil
}

func removeObsolete(plan *domain.Plan, workspace string) error {
	currentPaths := buildCurrentPathSet(plan)

	if plan.OldLock == nil {
		return nil
	}

	err := removeStaleManagedFiles(plan.OldLock, currentPaths, workspace)
	if err != nil {
		return fmt.Errorf("remove stale managed files: %w", err)
	}

	err = cleanupOldTarget(plan, workspace)
	if err != nil {
		return fmt.Errorf("clean up old target: %w", err)
	}

	return nil
}

func removeObsoleteFile(workspace, path string) error {
	abs := pathutil.WorkspacePath(workspace, path)

	err := os.Remove(abs)

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(errFmtRemoveObsoleteFile, path, err)
	}

	return nil
}

func removeOldTargetFiles(plan *domain.Plan, workspace string) error {
	for i := range plan.OldLock.ManagedFiles {
		old := &plan.OldLock.ManagedFiles[i]

		if !pathutil.HasFolderPrefix(old.Path, plan.OldTargetFolder) {
			continue
		}

		err := removeIfExists(workspace, old.Path)
		if err != nil {
			return fmt.Errorf("remove old target file %q: %w", old.Path, err)
		}
	}

	return nil
}

func removeOldTargetLock(plan *domain.Plan, workspace string) error {
	err := removeIfExists(
		workspace,
		pathutil.JoinRelative(plan.OldTargetFolder, lockFileName),
	)
	if err != nil {
		return fmt.Errorf("remove old lock file: %w", err)
	}

	return nil
}

func removeOldTargetMetadata(plan *domain.Plan, workspace string) error {
	oldMetadataRel := pathutil.JoinRelative(plan.OldTargetFolder, legacyMetadataRelPath)

	err := removeIfExists(workspace, oldMetadataRel)
	if err != nil {
		return fmt.Errorf("remove old metadata file: %w", err)
	}

	oldMetadata := pathutil.WorkspacePath(workspace, oldMetadataRel)

	err = removeDirIfEmpty(filepath.Dir(oldMetadata), "remove old metadata directory")
	if err != nil {
		return fmt.Errorf("clean up old metadata directory: %w", err)
	}

	return nil
}

func pruneEmptyParentDirs(workspace, fileRel, stopRel string) error {
	if stopRel == consts.Empty {
		return nil
	}

	err := pruneDirsUntilStop(
		filepath.Dir(pathutil.WorkspacePath(workspace, fileRel)),
		pathutil.WorkspacePath(workspace, stopRel),
	)
	if err != nil {
		return fmt.Errorf("prune empty parent dirs: %w", err)
	}

	return nil
}

func pruneDirsUntilStop(dir, stop string) error {
	for shouldPruneParent(dir, stop) {
		stillExists, err := removeEmptyParentDir(dir)
		if err != nil {
			return fmt.Errorf("prune dirs until stop: %w", err)
		}

		if stillExists {
			return nil
		}

		dir = filepath.Dir(dir)
	}

	return nil
}

func shouldPruneParent(dir, stop string) bool {
	if dir == stop || filepath.Dir(dir) == dir {
		return false
	}

	rel, relErr := filepath.Rel(stop, dir)

	return relErr == nil && filepath.IsLocal(rel)
}

func removeEmptyParentDir(dir string) (bool, error) {
	err := removeDirIfEmpty(dir, "remove empty parent directory")
	if err != nil {
		return false, fmt.Errorf("prune empty parents: %w", err)
	}

	return pathPresent(dir), nil
}

func pathPresent(path string) bool {
	info, err := os.Stat(path)
	iox.Discard(info)

	return err == nil
}

func removeStaleManagedFile(args *removeStaleFileArgs) error {
	if _, managed := args.current[args.old.Path]; managed {
		return nil
	}

	err := removeObsoleteFile(args.workspace, args.old.Path)
	if err != nil {
		return fmt.Errorf(errFmtRemoveStaleManaged, err)
	}

	err = pruneModuleParents(args)
	if err != nil {
		return fmt.Errorf(errFmtRemoveStaleManaged, err)
	}

	return nil
}

func pruneModuleParents(args *removeStaleFileArgs) error {
	moduleRoot := pathutil.JoinRelative(args.targetFolder, args.old.DestinationModule)

	err := pruneEmptyParentDirs(args.workspace, args.old.Path, moduleRoot)
	if err != nil {
		return fmt.Errorf("prune empty parents after removing %q: %w", args.old.Path, err)
	}

	return nil
}

func removeStaleManagedFiles(
	lock *syncLock,
	current map[string]struct{},
	workspace string,
) error {
	for i := range lock.ManagedFiles {
		err := removeStaleManagedFile(&removeStaleFileArgs{
			old:          &lock.ManagedFiles[i],
			current:      current,
			workspace:    workspace,
			targetFolder: lock.Configuration.TargetFolder,
		})
		if err != nil {
			return fmt.Errorf(errFmtRemoveObsoleteFile, lock.ManagedFiles[i].Path, err)
		}
	}

	return nil
}

func resolveCopyHook(plan *domain.Plan) func(string, *domain.FileEntry) error {
	if plan.CopyFileTo != nil {
		return plan.CopyFileTo
	}

	return copyFileTo
}

func sortedContentRels(contents map[string]domain.FileEntry) []string {
	rels := make([]string, consts.IndexZero, len(contents))

	for rel := range contents {
		rels = append(rels, rel)
	}

	slices.Sort(rels)

	return rels
}

func stageContentRels(
	rels []string,
	contents map[string]domain.FileEntry,
	dest string,
) []stagedFile {
	staged := make([]stagedFile, consts.IndexZero, len(rels))

	for i := range rels {
		rel := rels[i]
		finalRel := pathutil.JoinRelative(dest, rel)

		staged = append(staged, stagedFile{finalRel: finalRel, entry: contents[rel]})
	}

	return staged
}

func stageLockAndMetadata(plan *domain.Plan, metadataPath string) ([]stagedFile, error) {
	lockBytes, err := MarshalLock(&plan.Lock)
	if err != nil {
		return nil, fmt.Errorf(errMarshalLockFile, err)
	}

	metaBytes, err := MarshalMetadata(&plan.Metadata)
	if err != nil {
		return nil, fmt.Errorf(errMarshalMetadata, err)
	}

	return []stagedFile{
		{
			finalRel: plan.Metadata.LockFile,
			entry:    domain.FileEntry{Data: lockBytes, Mode: fileModeRegular},
		},
		{
			finalRel: metadataPath,
			entry:    domain.FileEntry{Data: metaBytes, Mode: fileModeRegular},
		},
	}, nil
}

func stageModuleFiles(plan *domain.Plan, targetFolder string) []stagedFile {
	records := sortedModuleRecords(plan.Requested, plan.Dependencies)
	staged := make([]stagedFile, consts.IndexZero, len(records)*consts.IndexTwo)

	for i := range records {
		mod := &records[i]

		staged = append(
			staged,
			stageOneModule(mod, plan.ModuleContents[mod.SourceModule], targetFolder)...,
		)
	}

	return staged
}

func stageOneModule(
	mod *moduleRecord,
	contents map[string]domain.FileEntry,
	tgt string,
) []stagedFile {
	rels := sortedContentRels(contents)
	destDirRel := pathutil.JoinRelative(tgt, mod.DestinationModule)

	return stageContentRels(rels, contents, destDirRel)
}

func stagePlanFiles(args *stagePlanArgs) (string, error) {
	stagingRoot, err := prepareStagingRoot(args.workspace, args.targetFolder)
	if err != nil {
		return consts.Empty, fmt.Errorf("prepare staging root: %w", err)
	}

	err = copyStagedFiles(stagingRoot, args.staged, args.copyFile)
	if err != nil {
		stagingErr := cleanupFailedStaging(stagingRoot, err)

		return consts.Empty, fmt.Errorf("copy staged files: %w", stagingErr)
	}

	return stagingRoot, nil
}

func stagePreparedFiles(input *stagePreparedInput) (stagingSession, error) {
	copyFile := resolveCopyHook(input.plan)

	stagingRoot, err := stagePlanFiles(&stagePlanArgs{
		staged:       input.staged,
		workspace:    input.workspace,
		targetFolder: input.syncInput.Config.TargetFolder,
		copyFile:     copyFile,
	})
	if err != nil {
		return stagingSession{}, fmt.Errorf("stage plan files: %w", err)
	}

	return stagingSession{
		staged:      input.staged,
		copyFile:    copyFile,
		stagingRoot: stagingRoot,
	}, nil
}

func validateAndWriteStaged(input *validateWriteStagedInput) error {
	err := validateGeneratedYAML(input.args.staged, input.args.rootPath)
	if err != nil {
		return fmt.Errorf("validate generated yaml: %w", err)
	}

	err = writeStagedFiles(&writeStagedArgs{
		staged:    input.args.staged,
		workspace: input.workspace,
		copyFile:  input.copyFile,
	})
	if err != nil {
		return fmt.Errorf("write staged files: %w", err)
	}

	return nil
}

func validateGeneratedYAML(staged []stagedFile, rootPath string) error {
	for i := range staged {
		stagedEntry := &staged[i]

		err := validateStagedYAML(stagedEntry, rootPath)
		if err != nil {
			return fmt.Errorf("validate staged yaml %q: %w", stagedEntry.finalRel, err)
		}
	}

	return nil
}

func validateLockFileYAML(data []byte) error {
	var lock syncLock

	err := yaml.Unmarshal(data, &lock)
	if err != nil {
		return fmt.Errorf(errValidateLockFile, err)
	}

	return nil
}

func validateMetadataYAML(data []byte) error {
	var meta domain.Metadata

	err := yaml.Unmarshal(data, &meta)
	if err != nil {
		return fmt.Errorf(errValidateMetadata, err)
	}

	return nil
}

func validateRootTaskfileYAML(data []byte) error {
	var node yaml.Node

	err := yaml.Unmarshal(data, &node)
	if err != nil {
		return fmt.Errorf(errValidateRootTaskfile, err)
	}

	return nil
}

func classifyStagedYAML(finalRel, rootPath string) yamlStagedKind {
	if finalRel == rootPath {
		return yamlStagedRoot
	}

	switch filepath.Base(finalRel) {
	case lockFileName:
		return yamlStagedLock
	case storeMetadataFileName:
		return yamlStagedMetadata
	default:
		return yamlStagedSkip
	}
}

func validateStagedYAMLData(kind yamlStagedKind, data []byte) error {
	validate := yamlValidatorForKind(kind)

	if validate == nil {
		return nil
	}

	err := validate(data)
	if err != nil {
		return fmt.Errorf("validate staged yaml: %w", err)
	}

	return nil
}

func yamlValidatorForKind(kind yamlStagedKind) func([]byte) error {
	if kind == yamlStagedRoot {
		return validateRootTaskfileYAML
	}

	if kind == yamlStagedLock {
		return validateLockFileYAML
	}

	if kind == yamlStagedMetadata {
		return validateMetadataYAML
	}

	return nil
}

func validateStagedYAML(stagedEntry *stagedFile, rootPath string) error {
	err := validateStagedYAMLData(
		classifyStagedYAML(stagedEntry.finalRel, rootPath),
		stagedEntry.entry.Data,
	)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	return nil
}

func writeStagedFiles(args *writeStagedArgs) error {
	for i := range args.staged {
		stagedEntry := &args.staged[i]
		finalPath := pathutil.WorkspacePath(args.workspace, stagedEntry.finalRel)

		err := os.MkdirAll(filepath.Dir(finalPath), dirModePerm)
		if err != nil {
			return fmt.Errorf("prepare %q: %w", stagedEntry.finalRel, err)
		}

		err = args.copyFile(finalPath, &stagedEntry.entry)
		if err != nil {
			return fmt.Errorf(errFmtWriteQuoted, stagedEntry.finalRel, err)
		}
	}

	return nil
}
