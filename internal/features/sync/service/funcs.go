// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	resolvesvc "github.com/task-otter/Taskotter/internal/features/resolve/service"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/features/sync/ports"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	yaml "go.yaml.in/yaml/v3"
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
	session, err := startApplySessionWithOps(defaultFileOps(), plan, syncInput)
	if err != nil {
		return fmt.Errorf("start apply session: %w", err)
	}

	defer cleanupStagingOnExitWithOps(session.fsOps, session.stagingRoot, &err)

	err = executeAppliedPlan(&session, plan, syncInput)
	if err != nil {
		return fmt.Errorf("execute applied plan: %w", err)
	}

	return nil
}

func cleanupStagingOnExit(stagingRoot string, err *error) {
	cleanupStagingOnExitWithOps(defaultFileOps(), stagingRoot, err)
}

func cleanupStagingOnExitWithOps(ops *fileOps, stagingRoot string, err *error) {
	cleanupErr := cleanupStagingDirWithOps(ops, stagingRoot)

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
	return startApplySessionWithOps(defaultFileOps(), plan, syncInput)
}

func startApplySessionWithOps(
	ops *fileOps,
	plan *domain.Plan,
	syncInput *domain.SyncInput,
) (stagingSession, error) {
	session, err := prepareStaging(&prepareStagingInput{
		plan:      plan,
		syncInput: syncInput,
		workspace: syncInput.Config.Workspace,
		fsOps:     ops,
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
		fsOps:     input.session.fsOps,
	})
	if err != nil {
		return fmt.Errorf("validate and write staged files: %w", err)
	}

	err = cleanupAfterApplyPlanWithOps(input, input.session.fsOps)
	if err != nil {
		return fmt.Errorf("cleanup after apply plan: %w", err)
	}

	return nil
}

func cleanupAfterApplyPlan(input *applyStagedInput) error {
	return cleanupAfterApplyPlanWithOps(input, defaultFileOps())
}

func cleanupAfterApplyPlanWithOps(input *applyStagedInput, ops *fileOps) error {
	err := cleanupAfterApplyWithOps(
		ops,
		input.plan,
		input.workspace,
		config.MetadataPath(input.syncInput.Config),
	)
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

func buildStagedFiles(plan *domain.Plan, syncInput *domain.SyncInput) []stagedFile {
	staged := stageModuleFiles(plan, syncInput.Config.TargetFolder)

	if syncInput.Config.SyncRoot {
		staged = append(staged, stagedFile{
			finalRel: plan.RootTaskfilePath,
			entry:    domain.FileEntry{Data: plan.RootTaskfile, Mode: fileModeRegular},
		})
	}

	lockAndMeta := stageLockAndMetadata(plan, config.MetadataPath(syncInput.Config))

	return append(staged, lockAndMeta...)
}

func cleanupAfterApply(plan *domain.Plan, workspace, metadataPath string) error {
	return cleanupAfterApplyWithOps(defaultFileOps(), plan, workspace, metadataPath)
}

func cleanupAfterApplyWithOps(
	ops *fileOps,
	plan *domain.Plan,
	workspace, metadataPath string,
) error {
	err := removeObsoleteWithOps(ops, plan, workspace)
	if err != nil {
		return fmt.Errorf("remove obsolete files: %w", err)
	}

	err = cleanupLegacyMetadataWithOps(ops, workspace, metadataPath)
	if err != nil {
		return fmt.Errorf("clean up legacy metadata: %w", err)
	}

	return nil
}

func cleanupFailedStaging(stagingRoot string, copyErr error) error {
	return cleanupFailedStagingWithOps(defaultFileOps(), stagingRoot, copyErr)
}

func cleanupFailedStagingWithOps(ops *fileOps, stagingRoot string, copyErr error) error {
	removeErr := withFileOps(ops).removeAll(stagingRoot)
	if removeErr != nil {
		return errors.Join(
			copyErr,
			fmt.Errorf(errFmtCleanupStagingDir, stagingRoot, removeErr),
		)
	}

	return copyErr
}

func cleanupLegacyMetadata(workspace, metadataPath string) error {
	return cleanupLegacyMetadataWithOps(defaultFileOps(), workspace, metadataPath)
}

func cleanupLegacyMetadataWithOps(ops *fileOps, workspace, metadataPath string) error {
	if metadataPath == config.LegacyMetadataPath {
		return nil
	}

	err := removeLegacyMetadataFileWithOps(ops, workspace)
	if err != nil {
		return fmt.Errorf("remove legacy metadata file: %w", err)
	}

	err = removeLegacyMetadataDirWithOps(ops, workspace)
	if err != nil {
		return fmt.Errorf("remove legacy metadata directory: %w", err)
	}

	return nil
}

func cleanupOldTarget(plan *domain.Plan, workspace string) error {
	return cleanupOldTargetWithOps(defaultFileOps(), plan, workspace)
}

func cleanupOldTargetWithOps(ops *fileOps, plan *domain.Plan, workspace string) error {
	if oldTargetUnchanged(plan) {
		return nil
	}

	err := runOldTargetCleanupStepsWithOps(ops, plan, workspace)
	if err != nil {
		return fmt.Errorf("run old target cleanup steps: %w", err)
	}

	return nil
}

func runOldTargetCleanupSteps(plan *domain.Plan, workspace string) error {
	return runOldTargetCleanupStepsWithOps(defaultFileOps(), plan, workspace)
}

func runOldTargetCleanupStepsWithOps(ops *fileOps, plan *domain.Plan, workspace string) error {
	steps := []func(*domain.Plan, string) error{
		func(plan *domain.Plan, workspace string) error {
			return removeOldTargetFilesWithOps(ops, plan, workspace)
		},
		func(plan *domain.Plan, workspace string) error {
			return removeOldTargetLockWithOps(ops, plan, workspace)
		},
		func(plan *domain.Plan, workspace string) error {
			return removeOldTargetMetadataWithOps(ops, plan, workspace)
		},
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
	return cleanupStagingDirWithOps(defaultFileOps(), stagingRoot)
}

func cleanupStagingDirWithOps(ops *fileOps, stagingRoot string) error {
	removeErr := withFileOps(ops).removeAll(stagingRoot)
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
	session, err := stagePreparedFiles(&stagePreparedInput{
		staged:    buildStagedFiles(input.plan, input.syncInput),
		plan:      input.plan,
		syncInput: input.syncInput,
		workspace: input.workspace,
		fsOps:     input.fsOps,
	})
	if err != nil {
		return stagingSession{}, fmt.Errorf("stage prepared files: %w", err)
	}

	return session, nil
}

func prepareStagingRoot(workspace, targetFolder string) (string, error) {
	return prepareStagingRootWithOps(defaultFileOps(), workspace, targetFolder)
}

func prepareStagingRootWithOps(ops *fileOps, workspace, targetFolder string) (string, error) {
	ops = withFileOps(ops)

	stagingParent := pathutil.WorkspacePath(
		workspace,
		pathutil.JoinRelative(targetFolder, ".taskotter/staging"),
	)

	err := ops.mkdirAll(stagingParent, dirModePerm)
	if err != nil {
		return consts.Empty, fmt.Errorf(errCreateStagingDir, err)
	}

	stagingRoot, err := ops.mkdirTemp(stagingParent, "apply-*")
	if err != nil {
		return consts.Empty, fmt.Errorf(errCreateStagingDir, err)
	}

	return stagingRoot, nil
}

func removeDirIfEmpty(dir, context string) error {
	return removeDirIfEmptyWithOps(defaultFileOps(), dir, context)
}

func removeDirIfEmptyWithOps(ops *fileOps, dir, context string) error {
	err := withFileOps(ops).removePath(dir)

	if err != nil && !os.IsNotExist(err) && !errorsIsDirectoryNotEmpty(err) {
		return fmt.Errorf("%s: %w", context, err)
	}

	return nil
}

func removeIfExists(workspace, rel string) error {
	return removeIfExistsWithOps(defaultFileOps(), workspace, rel)
}

func removeIfExistsWithOps(ops *fileOps, workspace, rel string) error {
	err := withFileOps(ops).removePath(pathutil.WorkspacePath(workspace, rel))

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %q: %w", rel, err)
	}

	return nil
}

func removeLegacyMetadataDir(workspace string) error {
	return removeLegacyMetadataDirWithOps(defaultFileOps(), workspace)
}

func removeLegacyMetadataDirWithOps(ops *fileOps, workspace string) error {
	legacyDir := pathutil.WorkspacePath(workspace, legacyMetadataDirName)

	err := removeDirIfEmptyWithOps(ops, legacyDir, "remove legacy metadata directory")
	if err != nil {
		return fmt.Errorf("clean up legacy metadata directory: %w", err)
	}

	return nil
}

func removeLegacyMetadataFile(workspace string) error {
	return removeLegacyMetadataFileWithOps(defaultFileOps(), workspace)
}

func removeLegacyMetadataFileWithOps(ops *fileOps, workspace string) error {
	legacy := pathutil.WorkspacePath(workspace, config.LegacyMetadataPath)

	err := withFileOps(ops).removePath(legacy)

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy metadata: %w", err)
	}

	return nil
}

func removeObsolete(plan *domain.Plan, workspace string) error {
	return removeObsoleteWithOps(defaultFileOps(), plan, workspace)
}

func removeObsoleteWithOps(ops *fileOps, plan *domain.Plan, workspace string) error {
	currentPaths := buildCurrentPathSet(plan)

	if plan.OldLock == nil {
		return nil
	}

	err := removeStaleManagedFilesWithOps(ops, plan.OldLock, currentPaths, workspace)
	if err != nil {
		return fmt.Errorf("remove stale managed files: %w", err)
	}

	err = cleanupOldTargetWithOps(ops, plan, workspace)
	if err != nil {
		return fmt.Errorf("clean up old target: %w", err)
	}

	return nil
}

func removeObsoleteFile(workspace, relPath string) error {
	return removeObsoleteFileWithOps(defaultFileOps(), workspace, relPath)
}

func removeObsoleteFileWithOps(ops *fileOps, workspace, relPath string) error {
	abs := pathutil.WorkspacePath(workspace, relPath)

	err := withFileOps(ops).removePath(abs)

	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(errFmtRemoveObsoleteFile, relPath, err)
	}

	return nil
}

func removeOldTargetFiles(plan *domain.Plan, workspace string) error {
	return removeOldTargetFilesWithOps(defaultFileOps(), plan, workspace)
}

func removeOldTargetFilesWithOps(ops *fileOps, plan *domain.Plan, workspace string) error {
	for i := range plan.OldLock.ManagedFiles {
		old := &plan.OldLock.ManagedFiles[i]

		if !pathutil.HasFolderPrefix(old.Path, plan.OldTargetFolder) {
			continue
		}

		err := removeIfExistsWithOps(ops, workspace, old.Path)
		if err != nil {
			return fmt.Errorf("remove old target file %q: %w", old.Path, err)
		}
	}

	return nil
}

func removeOldTargetLock(plan *domain.Plan, workspace string) error {
	return removeOldTargetLockWithOps(defaultFileOps(), plan, workspace)
}

func removeOldTargetLockWithOps(ops *fileOps, plan *domain.Plan, workspace string) error {
	err := removeIfExistsWithOps(
		ops,
		workspace,
		pathutil.JoinRelative(plan.OldTargetFolder, lockFileName),
	)
	if err != nil {
		return fmt.Errorf("remove old lock file: %w", err)
	}

	return nil
}

func removeOldTargetMetadata(plan *domain.Plan, workspace string) error {
	return removeOldTargetMetadataWithOps(defaultFileOps(), plan, workspace)
}

func removeOldTargetMetadataWithOps(ops *fileOps, plan *domain.Plan, workspace string) error {
	oldMetadataRel := pathutil.JoinRelative(plan.OldTargetFolder, legacyMetadataRelPath)

	err := removeIfExistsWithOps(ops, workspace, oldMetadataRel)
	if err != nil {
		return fmt.Errorf("remove old metadata file: %w", err)
	}

	oldMetadata := pathutil.WorkspacePath(workspace, oldMetadataRel)

	err = removeDirIfEmptyWithOps(ops, filepath.Dir(oldMetadata), "remove old metadata directory")
	if err != nil {
		return fmt.Errorf("clean up old metadata directory: %w", err)
	}

	return nil
}

func pruneEmptyParentDirs(workspace, fileRel, stopRel string) error {
	return pruneEmptyParentDirsWithOps(defaultFileOps(), workspace, fileRel, stopRel)
}

func pruneEmptyParentDirsWithOps(ops *fileOps, workspace, fileRel, stopRel string) error {
	if stopRel == consts.Empty {
		return nil
	}

	err := pruneDirsUntilStopWithOps(ops,
		filepath.Dir(pathutil.WorkspacePath(workspace, fileRel)),
		pathutil.WorkspacePath(workspace, stopRel),
	)
	if err != nil {
		return fmt.Errorf("prune empty parent dirs: %w", err)
	}

	return nil
}

func pruneDirsUntilStop(dir, stop string) error {
	return pruneDirsUntilStopWithOps(defaultFileOps(), dir, stop)
}

func pruneDirsUntilStopWithOps(ops *fileOps, dir, stop string) error {
	for shouldPruneParent(dir, stop) {
		stillExists, err := removeEmptyParentDirWithOps(ops, dir)
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
	return removeEmptyParentDirWithOps(defaultFileOps(), dir)
}

func removeEmptyParentDirWithOps(ops *fileOps, dir string) (bool, error) {
	err := removeDirIfEmptyWithOps(ops, dir, "remove empty parent directory")
	if err != nil {
		return false, fmt.Errorf("prune empty parents: %w", err)
	}

	return pathPresentWithOps(ops, dir), nil
}

func pathPresent(filePath string) bool {
	return pathPresentWithOps(defaultFileOps(), filePath)
}

func pathPresentWithOps(ops *fileOps, filePath string) bool {
	info, err := withFileOps(ops).statPath(filePath)
	iox.Discard(info)

	return err == nil
}

func removeStaleManagedFile(args *removeStaleFileArgs) error {
	if _, managed := args.current[args.old.Path]; managed {
		return nil
	}

	err := removeObsoleteFileWithOps(args.fsOps, args.workspace, args.old.Path)
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

	err := pruneEmptyParentDirsWithOps(args.fsOps, args.workspace, args.old.Path, moduleRoot)
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
	return removeStaleManagedFilesWithOps(defaultFileOps(), lock, current, workspace)
}

func removeStaleManagedFilesWithOps(
	ops *fileOps,
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
			fsOps:        ops,
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

func stageLockAndMetadata(plan *domain.Plan, metadataPath string) []stagedFile {
	lockBytes := MarshalLock(&plan.Lock)
	metaBytes := MarshalMetadata(&plan.Metadata)

	return []stagedFile{
		{
			finalRel: plan.Metadata.LockFile,
			entry:    domain.FileEntry{Data: lockBytes, Mode: fileModeRegular},
		},
		{
			finalRel: metadataPath,
			entry:    domain.FileEntry{Data: metaBytes, Mode: fileModeRegular},
		},
	}
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
	stagingRoot, err := prepareStagingRootWithOps(args.fsOps, args.workspace, args.targetFolder)
	if err != nil {
		return consts.Empty, fmt.Errorf("prepare staging root: %w", err)
	}

	err = copyStagedFiles(stagingRoot, args.staged, args.copyFile)
	if err != nil {
		stagingErr := cleanupFailedStagingWithOps(args.fsOps, stagingRoot, err)

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
		fsOps:        input.fsOps,
	})
	if err != nil {
		return stagingSession{}, fmt.Errorf("stage plan files: %w", err)
	}

	return stagingSession{
		staged:      input.staged,
		copyFile:    copyFile,
		stagingRoot: stagingRoot,
		fsOps:       input.fsOps,
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
		fsOps:     input.fsOps,
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

	err := lockmodel.DecodeLockFileYAML(data, &lock)
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
	ops := withFileOps(args.fsOps)

	for i := range args.staged {
		stagedEntry := &args.staged[i]
		finalPath := pathutil.WorkspacePath(args.workspace, stagedEntry.finalRel)

		err := writeOneStagedFile(ops, args.copyFile, finalPath, stagedEntry)
		if err != nil {
			return fmt.Errorf("stage %q: %w", stagedEntry.finalRel, err)
		}
	}

	return nil
}

func writeOneStagedFile(
	ops *fileOps,
	copyFile func(string, *domain.FileEntry) error,
	finalPath string,
	entry *stagedFile,
) error {
	err := withFileOps(ops).mkdirAll(filepath.Dir(finalPath), dirModePerm)
	if err != nil {
		return fmt.Errorf("prepare %q: %w", entry.finalRel, err)
	}

	err = copyFile(finalPath, &entry.entry)
	if err != nil {
		return fmt.Errorf(errFmtWriteQuoted, entry.finalRel, err)
	}

	return nil
}

func lockContentChanged(old, newLock *syncLock) (changed bool, err error) {
	return lockContentChangedWithOps(defaultFileOps(), old, newLock)
}

func lockContentChangedWithOps(ops *fileOps, old, newLock *syncLock) (changed bool, err error) {
	oldNorm, err := marshalLockForCompareWithOps(ops, old)
	if err != nil {
		return false, fmt.Errorf("normalize old lock file: %w", err)
	}

	newNorm, err := marshalLockForCompareWithOps(ops, newLock)
	if err != nil {
		return false, fmt.Errorf("normalize new lock file: %w", err)
	}

	return !bytes.Equal(oldNorm, newNorm), nil
}

func marshalLockForCompare(lock *syncLock) ([]byte, error) {
	return marshalLockForCompareWithOps(defaultFileOps(), lock)
}

func marshalLockForCompareWithOps(ops *fileOps, lock *syncLock) ([]byte, error) {
	if lock == nil {
		return nil, nil
	}

	cloned := *lock

	cloned.Source.ResolvedCommit = consts.Empty

	data, err := withFileOps(ops).marshalYAML(lockmodel.EncodeLockFile(&cloned))
	if err != nil {
		return nil, fmt.Errorf("marshal lock file for compare: %w", err)
	}

	return data, nil
}

func diffFiles(input *diffInput) (diffLists, error) {
	current, removed := currentAndRemoved(input.plan)

	lists, err := diffManagedFilePaths(current, input.workspace)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff managed files: %w", err)
	}

	lists.removed = removed
	lists = *diffRootIfEnabled(input, &lists)

	lists, err = diffLockAndMetadata(input, &lists)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff lock and metadata: %w", err)
	}

	return *sortDiffLists(&lists), nil
}

func diffRootIfEnabled(input *diffInput, lists *diffLists) *diffLists {
	if input.syncRoot != syncRootEnabled {
		return lists
	}

	return diffRootTaskfile(&diffRootInput{
		oldRoot:  input.oldRoot,
		newRoot:  input.plan.RootTaskfile,
		rootPath: input.plan.RootTaskfilePath,
		lists:    *lists,
	})
}

func currentAndRemoved(plan *domain.Plan) (current map[string]managedFile, removed []string) {
	current = buildCurrentManagedFiles(plan)
	removed = diffRemovedFiles(plan.OldLock, current)

	return current, removed
}

func buildCurrentManagedFiles(plan *domain.Plan) map[string]managedFile {
	current := make(map[string]managedFile, len(plan.ManagedFiles))

	for i := range plan.ManagedFiles {
		managed := &plan.ManagedFiles[i]

		current[managed.Path] = *managed
	}

	return current
}

func diffRemovedFiles(oldLock *syncLock, current map[string]managedFile) []string {
	var removed []string

	if oldLock == nil {
		return removed
	}

	for i := range oldLock.ManagedFiles {
		managed := &oldLock.ManagedFiles[i]

		existing, ok := current[managed.Path]
		iox.Discard(existing)

		if !ok {
			removed = append(removed, managed.Path)
		}
	}

	return removed
}

func diffLockAndMetadata(input *diffInput, lists *diffLists) (diffLists, error) {
	listsResult, err := diffLockFileSection(input, lists)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff lock file section: %w", err)
	}

	listsResult, err = diffMetadataFileSection(input, &listsResult)
	if err != nil {
		return diffLists{}, fmt.Errorf("diff metadata file section: %w", err)
	}

	return listsResult, nil
}

func diffLockFileSection(input *diffInput, lists *diffLists) (diffLists, error) {
	listsResult, err := diffLockFile(&diffLockArgs{
		plan:      input.plan,
		workspace: input.workspace,
		lockPath:  input.plan.Metadata.LockFile,
		lists:     *lists,
		fsOps:     input.fsOps,
	})
	if err != nil {
		return diffLists{}, fmt.Errorf("diff lock file: %w", err)
	}

	return listsResult, nil
}

func diffMetadataFileSection(input *diffInput, lists *diffLists) (diffLists, error) {
	listsResult, err := diffMetadataFile(&diffMetadataArgs{
		workspace:    input.workspace,
		metadataPath: input.metadataPath,
		plannedMeta:  input.plannedMeta,
		lists:        *lists,
	})
	if err != nil {
		return diffLists{}, fmt.Errorf("diff metadata file: %w", err)
	}

	return listsResult, nil
}

func sortDiffLists(lists *diffLists) *diffLists {
	slices.Sort(lists.added)
	slices.Sort(lists.updated)
	slices.Sort(lists.removed)

	return lists
}

func diffManagedFilePaths(current map[string]managedFile, workspace string) (diffLists, error) {
	var lists diffLists

	for relPath := range current {
		managed := current[relPath]

		change, changeErr := fileChanged(workspace, relPath, &managed)
		if changeErr != nil {
			return diffLists{}, fmt.Errorf("read managed file %q: %w", relPath, changeErr)
		}

		lists = *applyFileChange(&lists, relPath, change)
	}

	return lists, nil
}

func applyFileChange(lists *diffLists, relPath string, change fileChangeKind) *diffLists {
	switch change {
	case fileUnchanged:
		return lists
	case fileAdded:
		lists.added = append(lists.added, relPath)
	case fileUpdated:
		lists.updated = append(lists.updated, relPath)
	}

	return lists
}

func fileChanged(workspace, relPath string, managed *managedFile) (fileChangeKind, error) {
	data, readErr := pathutil.ReadRelativeFile(workspace, relPath)
	if readErr == nil {
		return fileChangeFromData(data, managed), nil
	}

	if errors.Is(readErr, os.ErrNotExist) {
		return fileAdded, nil
	}

	return fileUnchanged, fmt.Errorf(errFmtReadQuoted, relPath, readErr)
}

func fileChangeFromData(data []byte, managed *managedFile) fileChangeKind {
	sum := sha256.Sum256(data)

	if hex.EncodeToString(sum[:]) != managed.SHA256 {
		return fileUpdated
	}

	return fileUnchanged
}

func diffRootTaskfile(input *diffRootInput) *diffLists {
	if bytes.Equal(input.oldRoot, input.newRoot) {
		return &input.lists
	}

	if len(input.oldRoot) == consts.IndexZero {
		input.lists.added = append(input.lists.added, input.rootPath)

		return &input.lists
	}

	input.lists.updated = append(input.lists.updated, input.rootPath)

	return &input.lists
}

func classifyAddedOrUpdated(prior priorContent, relPath string, lists *diffLists) *diffLists {
	if prior == priorContentEmpty {
		lists.added = append(lists.added, relPath)

		return lists
	}

	lists.updated = append(lists.updated, relPath)

	return lists
}

func priorContentFromBytes(data []byte) priorContent {
	if len(data) == consts.IndexZero {
		return priorContentEmpty
	}

	return priorContentExists
}

func diffLockFile(args *diffLockArgs) (diffLists, error) {
	oldLockBytes, readErr := pathutil.ReadRelativeFile(args.workspace, args.lockPath)

	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return diffLists{}, fmt.Errorf(errReadLockFile, args.lockPath, readErr)
	}

	changed, err := lockContentChangedWithOps(args.fsOps, args.plan.OldLock, &args.plan.Lock)
	if err != nil {
		return diffLists{}, fmt.Errorf("lock content changed: %w", err)
	}

	if !changed {
		return args.lists, nil
	}

	return *classifyAddedOrUpdated(
		priorContentFromBytes(oldLockBytes),
		args.lockPath,
		&args.lists,
	), nil
}

func diffMetadataFile(args *diffMetadataArgs) (diffLists, error) {
	oldMetaBytes, readErr := pathutil.ReadRelativeFile(args.workspace, args.metadataPath)

	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return diffLists{}, fmt.Errorf(errReadMetadata, args.metadataPath, readErr)
	}

	if bytes.Equal(oldMetaBytes, args.plannedMeta) {
		return args.lists, nil
	}

	return *classifyAddedOrUpdated(
		priorContentFromBytes(oldMetaBytes),
		args.metadataPath,
		&args.lists,
	), nil
}

func buildStagePaths(input *stagePathsInput) []string {
	paths := baseStagePaths(input.plan, input.syncRoot)

	addMetadataStagePaths(paths, input.workspace, input.metadataPath)
	addOldTargetStagePaths(paths, input.plan, input.workspace)

	return sortedStagePaths(paths)
}

func baseStagePaths(plan *domain.Plan, syncRoot syncRootPolicy) map[string]struct{} {
	paths := make(map[string]struct{})

	for i := range plan.ManagedFiles {
		paths[plan.ManagedFiles[i].Path] = struct{}{}
	}

	for i := range plan.Removed {
		paths[plan.Removed[i]] = struct{}{}
	}

	if syncRoot == syncRootEnabled {
		paths[plan.RootTaskfilePath] = struct{}{}
	}

	paths[plan.Metadata.LockFile] = struct{}{}

	return paths
}

func addMetadataStagePaths(paths map[string]struct{}, workspace, metadataPath string) {
	paths[metadataPath] = struct{}{}

	if shouldStageLegacyMetadata(workspace, metadataPath) {
		paths[config.LegacyMetadataPath] = struct{}{}
	}
}

func shouldStageLegacyMetadata(workspace, metadataPath string) bool {
	return metadataPath != config.LegacyMetadataPath &&
		relativePathExists(workspace, config.LegacyMetadataPath)
}

func addOldTargetStagePaths(paths map[string]struct{}, plan *domain.Plan, workspace string) {
	if oldTargetUnchanged(plan) {
		return
	}

	addOldManagedFilePaths(paths, plan.OldLock, plan.OldTargetFolder)

	oldLockPath := pathutil.JoinRelative(plan.OldTargetFolder, lockFileName)

	paths[oldLockPath] = struct{}{}

	oldMetadataPath := pathutil.JoinRelative(plan.OldTargetFolder, legacyMetadataRelPath)

	if relativePathExists(workspace, oldMetadataPath) {
		paths[oldMetadataPath] = struct{}{}
	}
}

func addOldManagedFilePaths(paths map[string]struct{}, oldLock *syncLock, oldTargetFolder string) {
	for i := range oldLock.ManagedFiles {
		managed := &oldLock.ManagedFiles[i]

		if pathutil.HasFolderPrefix(managed.Path, oldTargetFolder) {
			paths[managed.Path] = struct{}{}
		}
	}
}

func sortedStagePaths(paths map[string]struct{}) []string {
	out := make([]string, consts.IndexZero, len(paths))

	for relPath := range paths {
		out = append(out, relPath)
	}

	slices.Sort(out)

	return out
}

func relativePathExists(workspace, rel string) bool {
	return relativePathExistsWithOps(defaultFileOps(), workspace, rel)
}

func relativePathExistsWithOps(ops *fileOps, workspace, rel string) bool {
	info, err := withFileOps(ops).statPath(pathutil.WorkspacePath(workspace, rel))
	iox.Discard(info)

	return err == nil
}

// SetCopyFileToHookForTest installs a copy hook on plan for tests.
func SetCopyFileToHookForTest(plan *domain.Plan, hook func(string, *domain.FileEntry) error) {
	plan.CopyFileTo = hook
}

func writeFileAtomic(filePath string, data []byte, mode os.FileMode) error {
	return writeFileAtomicWithOps(defaultFileOps(), filePath, data, mode)
}

func writeFileAtomicWithOps(ops *fileOps, filePath string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(filePath)

	tmp, err := createTempFileWithOps(ops, dir)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	err = finalizeTempFile(
		&finalizeTempArgs{tmp: tmp, data: data, mode: mode, path: filePath, fsOps: ops},
	)
	if err != nil {
		return fmt.Errorf("finalize temp file: %w", err)
	}

	return nil
}

func createTempFile(dir string) (*os.File, error) {
	return createTempFileWithOps(defaultFileOps(), dir)
}

func createTempFileWithOps(ops *fileOps, dir string) (*os.File, error) {
	ops = withFileOps(ops)

	err := ops.mkdirAll(dir, dirModePerm)
	if err != nil {
		return nil, fmt.Errorf("create directory %q: %w", dir, err)
	}

	tmp, err := ops.createTemp(dir, stagingTempPattern)
	if err != nil {
		return nil, fmt.Errorf("create temp file in %q: %w", dir, err)
	}

	return tmp, nil
}

func finalizeTempFile(args *finalizeTempArgs) error {
	args.fsOps = withFileOps(args.fsOps)

	tmpPath := args.tmp.Name()
	cleanup := true

	defer cleanupTempFileWithOps(args.fsOps, args.tmp, tmpPath, &cleanup)

	err := commitTempFile(args, tmpPath, &cleanup)
	if err != nil {
		return fmt.Errorf("commit temp file: %w", err)
	}

	return nil
}

func commitTempFile(args *finalizeTempArgs, tmpPath string, cleanup *bool) error {
	err := writeAndFinalizeTempWithOps(args.fsOps, args.tmp, args.data, args.mode)
	if err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	*cleanup = false

	err = renameTempFileWithOps(args.fsOps, tmpPath, args.path)
	if err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func cleanupTempFile(tmp *os.File, tmpPath string, cleanup *bool) {
	cleanupTempFileWithOps(defaultFileOps(), tmp, tmpPath, cleanup)
}

func cleanupTempFileWithOps(ops *fileOps, tmp *os.File, tmpPath string, cleanup *bool) {
	if *cleanup {
		ops = withFileOps(ops)

		closeErr := ops.closeFile(tmp)
		iox.Discard(closeErr)

		removeErr := ops.removePath(tmpPath)
		iox.Discard(removeErr)
	}
}

func writeAndFinalizeTemp(tmp *os.File, data []byte, mode os.FileMode) error {
	return writeAndFinalizeTempWithOps(defaultFileOps(), tmp, data, mode)
}

func writeAndFinalizeTempWithOps(ops *fileOps, tmp *os.File, data []byte, mode os.FileMode) error {
	err := writeTempData(withFileOps(ops), tmp, data)
	if err != nil {
		return err
	}

	err = chmodTempFile(withFileOps(ops), tmp, mode)
	if err != nil {
		return err
	}

	err = withFileOps(ops).closeFile(tmp)
	if err != nil {
		return fmt.Errorf("close temp file %q: %w", tmp.Name(), err)
	}

	return nil
}

func writeTempData(ops *fileOps, tmp *os.File, data []byte) error {
	err := ops.writeFull(tmp, data)
	if err != nil {
		return fmt.Errorf("write temp file %q: %w", tmp.Name(), err)
	}

	return nil
}

func chmodTempFile(ops *fileOps, tmp *os.File, mode os.FileMode) error {
	err := ops.chmodFile(tmp, mode)
	if err != nil {
		return fmt.Errorf("chmod temp file %q: %w", tmp.Name(), err)
	}

	return nil
}

func renameTempFile(tmpPath, filePath string) error {
	return renameTempFileWithOps(defaultFileOps(), tmpPath, filePath)
}

func renameTempFileWithOps(ops *fileOps, tmpPath, filePath string) error {
	err := withFileOps(ops).renamePath(tmpPath, filePath)
	if err != nil {
		return fmt.Errorf("rename temp file to %q: %w", filePath, err)
	}

	return nil
}

func copyFileTo(filePath string, entry *domain.FileEntry) error {
	return copyFileToWithOps(defaultFileOps(), filePath, entry)
}

func copyFileToWithOps(ops *fileOps, filePath string, entry *domain.FileEntry) error {
	err := writeFileAtomicWithOps(ops, filePath, entry.Data, entry.Mode)
	if err != nil {
		return fmt.Errorf("write file %q: %w", filePath, err)
	}

	return nil
}

// CopyFile copies rel under root to dst with the given mode.
func CopyFile(args *copyFileArgs) error {
	data, err := readRelativeFileWithOps(args.fsOps, args.root, args.rel)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	err = writeCopiedFileWithOps(args.fsOps, args.dst, data, args.mode)
	if err != nil {
		return fmt.Errorf("write copied file: %w", err)
	}

	return nil
}

func readRelativeFile(root, rel string) ([]byte, error) {
	return readRelativeFileWithOps(defaultFileOps(), root, rel)
}

func readRelativeFileWithOps(ops *fileOps, root, rel string) ([]byte, error) {
	source, err := openRelativeSource(ops, root, rel)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", rel, err)
	}

	defer func() {
		closeErr := ops.closeFile(source)
		iox.Discard(closeErr)
	}()

	data, err := readSourceData(ops, source, rel)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func openRelativeSource(ops *fileOps, root, rel string) (*os.File, error) {
	return withFileOps(ops).openRelativeFile(root, rel)
}

func readSourceData(ops *fileOps, source *os.File, rel string) ([]byte, error) {
	data, err := ops.readAll(source)
	if err != nil {
		return nil, fmt.Errorf(errFmtReadQuoted, rel, err)
	}

	return data, nil
}

func writeCopiedFile(dst string, data []byte, mode os.FileMode) error {
	return writeCopiedFileWithOps(defaultFileOps(), dst, data, mode)
}

func writeCopiedFileWithOps(ops *fileOps, dst string, data []byte, mode os.FileMode) error {
	err := writeFileAtomicWithOps(ops, dst, data, mode)
	if err != nil {
		return fmt.Errorf(errFmtWriteQuoted, dst, err)
	}

	return nil
}

func sortedModuleRecords(requested map[string]moduleRecord, deps []moduleRecord) []moduleRecord {
	tasks := make([]string, consts.IndexZero, len(requested))

	for task := range requested {
		tasks = append(tasks, task)
	}

	slices.Sort(tasks)

	out := make([]moduleRecord, consts.IndexZero, len(requested)+len(deps))

	for i := range tasks {
		out = append(out, requested[tasks[i]])
	}

	return append(out, deps...)
}

func preserveMode(mode os.FileMode) os.FileMode {
	perm := mode.Perm()

	if perm&consts.FilePerm111 != consts.IndexZero {
		return perm
	}

	return fileModeRegular
}

// LoadMetadata reads TaskOtter metadata from workspace-relative path rel.
func LoadMetadata(workspace, rel string) (*domain.Metadata, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf(errReadMetadata, rel, err)
	}

	var meta domain.Metadata

	err = yaml.Unmarshal(data, &meta)
	if err != nil {
		return nil, fmt.Errorf("parse metadata %q: %w", rel, err)
	}

	return &meta, nil
}

// LoadLock reads the TaskOtter lock file from workspace-relative path rel.
func LoadLock(workspace, rel string) (*syncLock, error) {
	data, err := pathutil.ReadRelativeFile(workspace, rel)
	if err != nil {
		return nil, fmt.Errorf(errReadLockFile, rel, err)
	}

	var lock syncLock

	err = lockmodel.DecodeLockFileYAML(data, &lock)
	if err != nil {
		return nil, fmt.Errorf("parse lock file %q: %w", rel, err)
	}

	return &lock, nil
}

func loadPreviousState(workspace string, cfg *config.Config) (previousState, error) {
	oldMeta, err := loadCurrentMetadata(workspace, cfg)

	if err != nil && !errors.Is(err, errPreviousMetadataNotFound) {
		return previousState{}, fmt.Errorf("load current metadata: %w", err)
	}

	oldLock, oldTarget, err := loadPreviousLock(workspace, cfg, oldMeta)
	if err != nil {
		return previousState{}, fmt.Errorf("load previous lock: %w", err)
	}

	return previousState{lock: oldLock, target: oldTarget}, nil
}

func loadCurrentMetadata(workspace string, cfg *config.Config) (*domain.Metadata, error) {
	meta, found, err := loadMetadataIfExists(workspace, config.MetadataPath(cfg))

	if err != nil && !errors.Is(err, errMetadataNotFound) {
		return nil, fmt.Errorf("load metadata: %w", err)
	}

	if found {
		return meta, nil
	}

	meta, err = loadMetadataFallbacks(workspace, config.MetadataPath(cfg))
	if err != nil {
		return nil, fmt.Errorf("load metadata fallbacks: %w", err)
	}

	return meta, nil
}

func loadMetadataFallbacks(workspace, currentMetadataPath string) (*domain.Metadata, error) {
	meta, found, err := tryLegacyMetadata(workspace, currentMetadataPath)

	if err != nil && !errors.Is(err, errMetadataNotFound) {
		return nil, fmt.Errorf("try legacy metadata: %w", err)
	}

	if found {
		return meta, nil
	}

	discovered, err := discoverPreviousMetadata(workspace, currentMetadataPath)
	if err != nil {
		return nil, fmt.Errorf(errDiscoverPreviousMetadata, err)
	}

	return discovered, nil
}

func tryLegacyMetadata(
	workspace, currentMetadataPath string,
) (meta *domain.Metadata, found bool, err error) {
	if currentMetadataPath == config.LegacyMetadataPath {
		return nil, false, errMetadataNotFound
	}

	meta, found, err = loadMetadataIfExists(workspace, config.LegacyMetadataPath)

	if err != nil && !errors.Is(err, errMetadataNotFound) {
		return nil, false, fmt.Errorf("load legacy metadata: %w", err)
	}

	if !found {
		return nil, false, errMetadataNotFound
	}

	return meta, true, nil
}

func loadMetadataIfExists(
	workspace, metadataPath string,
) (meta *domain.Metadata, found bool, err error) {
	meta, err = LoadMetadata(workspace, metadataPath)

	if errors.Is(err, os.ErrNotExist) {
		return nil, false, errMetadataNotFound
	}

	if err != nil {
		return nil, false, fmt.Errorf("load metadata %q: %w", metadataPath, err)
	}

	return meta, true, nil
}

func loadPreviousLock(
	workspace string,
	cfg *config.Config,
	oldMeta *domain.Metadata,
) (lock *syncLock, target string, err error) {
	resolved := resolveOldLockPathAndTarget(resolveLockArgs{cfg: cfg, oldMeta: oldMeta})

	oldLock, err := LoadLock(workspace, resolved.lockPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, consts.Empty, fmt.Errorf("load lock %q: %w", resolved.lockPath, err)
	}

	oldTarget := resolved.target

	if oldLock != nil && oldTarget == consts.Empty {
		oldTarget = oldLock.Configuration.TargetFolder
	}

	return oldLock, oldTarget, nil
}

func resolveOldLockPathAndTarget(args resolveLockArgs) lockPathResult {
	oldLockPath := config.LockFilePath(args.cfg)

	if args.oldMeta == nil {
		return lockPathResult{lockPath: oldLockPath, target: consts.Empty}
	}

	if args.oldMeta.LockFile != consts.Empty {
		oldLockPath = args.oldMeta.LockFile
	}

	return lockPathResult{lockPath: oldLockPath, target: args.oldMeta.TargetFolder}
}

func discoverPreviousMetadata(workspace, currentMetadataPath string) (*domain.Metadata, error) {
	return discoverPreviousMetadataWithOps(defaultFileOps(), workspace, currentMetadataPath)
}

func discoverPreviousMetadataWithOps(
	ops *fileOps,
	workspace, currentMetadataPath string,
) (*domain.Metadata, error) {
	candidates, err := collectMetadataCandidatesWithOps(ops, workspace, currentMetadataPath)
	if err != nil {
		return nil, fmt.Errorf(errDiscoverPreviousMetadata, err)
	}

	if len(candidates) == consts.IndexZero {
		return nil, errPreviousMetadataNotFound
	}

	meta, err := loadFirstCandidate(workspace, candidates)
	if err != nil {
		return nil, fmt.Errorf("load first candidate: %w", err)
	}

	return meta, nil
}

func collectMetadataCandidates(workspace, currentMetadataPath string) ([]string, error) {
	return collectMetadataCandidatesWithOps(defaultFileOps(), workspace, currentMetadataPath)
}

func collectMetadataCandidatesWithOps(
	ops *fileOps,
	workspace, currentMetadataPath string,
) ([]string, error) {
	var candidates []string

	walker := metadataCandidateWalker(&metadataWalkerArgs{
		workspace:           workspace,
		currentMetadataPath: currentMetadataPath,
		candidates:          &candidates,
		fsOps:               ops,
	})

	err := withFileOps(ops).walkDir(workspace, walker)
	if err != nil {
		return nil, fmt.Errorf(errDiscoverPreviousMetadata, err)
	}

	slices.Sort(candidates)

	return candidates, nil
}

func metadataCandidateWalker(args *metadataWalkerArgs) func(string, os.DirEntry, error) error {
	return func(abs string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf(errWalk, abs, walkErr)
		}

		return processMetadataCandidate(args, abs, entry)
	}
}

func processMetadataCandidate(args *metadataWalkerArgs, abs string, entry os.DirEntry) error {
	rel, scan, err := previousMetadataCandidate(&metadataCandidateArgs{
		workspace:           args.workspace,
		currentMetadataPath: args.currentMetadataPath,
		abs:                 abs,
		entry:               entry,
		fsOps:               args.fsOps,
	})
	if err == nil {
		recordMetadataCandidate(args.candidates, rel, scan)

		return nil
	}

	if errors.Is(err, filepath.SkipDir) {
		return filepath.SkipDir
	}

	return fmt.Errorf("previous metadata candidate: %w", err)
}

func recordMetadataCandidate(candidates *[]string, rel string, scan metadataScanResult) {
	if scan == metadataIsCandidate {
		*candidates = append(*candidates, rel)
	}
}

func loadFirstCandidate(workspace string, candidates []string) (*domain.Metadata, error) {
	meta, err := LoadMetadata(workspace, candidates[consts.IndexZero])
	if err != nil {
		return nil, fmt.Errorf("load previous metadata %q: %w", candidates[consts.IndexZero], err)
	}

	return meta, nil
}

func previousMetadataCandidate(args *metadataCandidateArgs) (string, metadataScanResult, error) {
	if args.entry.IsDir() {
		err := handleDirEntry(args.entry)
		if err != nil {
			return consts.Empty, metadataNotCandidate, fmt.Errorf(
				"handle metadata directory entry: %w",
				err,
			)
		}

		return consts.Empty, metadataNotCandidate, nil
	}

	rel, scan, err := metadataFileCandidate(args)
	if err != nil {
		return consts.Empty, metadataNotCandidate, fmt.Errorf("metadata file candidate: %w", err)
	}

	return rel, scan, nil
}

func metadataFileCandidate(args *metadataCandidateArgs) (string, metadataScanResult, error) {
	rel, err := relMetadataPathWithOps(args.fsOps, args.workspace, args.abs)
	if err != nil {
		return consts.Empty, metadataNotCandidate, fmt.Errorf("relative metadata path: %w", err)
	}

	if isCurrentOrLegacyMetadataPath(rel, args.currentMetadataPath) {
		return consts.Empty, metadataNotCandidate, nil
	}

	if isTaskOtterMetadataPath(rel) {
		return rel, metadataIsCandidate, nil
	}

	return consts.Empty, metadataNotCandidate, nil
}

func relMetadataPath(workspace, abs string) (string, error) {
	return relMetadataPathWithOps(defaultFileOps(), workspace, abs)
}

func relToSlashWithOps(ops *fileOps, base, target, contextName string) (string, error) {
	rel, err := withFileOps(ops).relPath(base, target)
	if err != nil {
		return consts.Empty, fmt.Errorf("%s for %q: %w", contextName, target, err)
	}

	return filepath.ToSlash(rel), nil
}

func relMetadataPathWithOps(ops *fileOps, workspace, abs string) (string, error) {
	return relToSlashWithOps(ops, workspace, abs, "relative metadata path")
}

func handleDirEntry(entry os.DirEntry) error {
	if entry.Name() == ".git" {
		return filepath.SkipDir
	}

	return nil
}

func isCurrentOrLegacyMetadataPath(rel, currentMetadataPath string) bool {
	return rel == currentMetadataPath || rel == config.LegacyMetadataPath
}

func isTaskOtterMetadataPath(rel string) bool {
	return filepath.Base(rel) == storeMetadataFileName &&
		filepath.Base(filepath.Dir(rel)) == legacyMetadataDirName
}

// BuildPlan computes the sync diff and generated artifacts for syncInput.
func BuildPlan(syncInput *domain.SyncInput) (*domain.Plan, error) {
	prev, err := loadPreviousState(syncInput.Config.Workspace, syncInput.Config)
	if err != nil {
		return nil, fmt.Errorf("load previous state: %w", err)
	}

	plan, err := buildPlanFromState(syncInput, &prev)
	if err != nil {
		return nil, fmt.Errorf("build plan from state: %w", err)
	}

	return plan, nil
}

// CollectModuleFiles scans a module source directory and returns syncable file entries.
func CollectModuleFiles(opts *CollectOptions) (map[string]domain.FileEntry, error) {
	contents, err := scanModuleFiles(collectOptionsFrom(opts))
	if err != nil {
		return nil, fmt.Errorf("scan module files: %w", err)
	}

	return contents, nil
}

func accumulateModulePlan(args *modulePlanArgs) ([]managedFile, error) {
	contents, managed, err := planModuleFiles(args)
	if err != nil {
		return nil, fmt.Errorf("plan module files: %w", err)
	}

	args.moduleContents[args.mod.SourceModule] = contents

	return append(args.planned, managed...), nil
}

func anyChanges(lists *diffLists) bool {
	return len(lists.added) > consts.IndexZero || len(lists.updated) > consts.IndexZero ||
		len(lists.removed) > consts.IndexZero
}

func emptyRootPlanResult() rootPlanResult {
	return rootPlanResult{
		rootBytes:          nil,
		newRoot:            nil,
		generatedRootTasks: nil,
		rootState:          rootAbsent,
	}
}

func appendManagedFiles(args *appendManagedArgs) []managedFile {
	for rel := range args.contents {
		entry := args.contents[rel]
		sum := sha256.Sum256(entry.Data)

		args.planned = append(args.planned, managedFile{
			SourceModule:      args.mod.SourceModule,
			DestinationModule: args.mod.DestinationModule,
			SourcePath:        managedSourcePath(args, rel),
			Path:              pathutil.JoinRelative(args.destDirRel, rel),
			SHA256:            hex.EncodeToString(sum[:]),
		})
	}

	return args.planned
}

func managedSourcePath(args *appendManagedArgs, rel string) string {
	sourceModule := args.mod.SourceModule

	if _, fromParent := args.parentDocs[rel]; fromParent {
		sourceModule = args.mod.DestinationModule
	}

	return pathutil.JoinRelative(taskfilesDirName, sourceModule, rel)
}

func applyDiffResults(plan *domain.Plan, lists *diffLists, input *stagePathsInput) {
	plan.Added = lists.added
	plan.Updated = lists.updated
	plan.Removed = lists.removed
	plan.Changed = anyChanges(lists)
	plan.StagePaths = buildStagePaths(input)
}

func assemblePlan(input *assemblePlanInput) *domain.Plan {
	return &domain.Plan{
		Requested:        input.syncInput.Requested,
		Dependencies:     input.syncInput.Dependencies,
		ManagedFiles:     input.artifacts.plannedFiles,
		ModuleContents:   input.artifacts.moduleContents,
		RootTaskfile:     input.artifacts.newRoot,
		RootTaskfilePath: input.syncInput.Config.RootTaskfile,
		Lock:             input.lock,
		Metadata:         *input.meta,
		OldLock:          input.prev.lock,
		OldTargetFolder:  input.prev.target,
		Updated:          nil,
		Removed:          nil,
		Added:            nil,
		StagePaths:       nil,
		Changed:          false,
		CopyFileTo:       nil,
	}
}

func buildFileEntry(args *fileEntryArgs) (domain.FileEntry, error) {
	info, err := args.entry.Info()
	if err != nil {
		return domain.FileEntry{}, fmt.Errorf("file info for %q: %w", args.absPath, err)
	}

	data, err := readAndMaybeRewriteModuleFile(&rewriteModuleArgs{
		ops:          args.ops,
		sourceDir:    args.sourceDir,
		fromDest:     args.fromDest,
		rel:          args.rel,
		absPath:      args.absPath,
		sourceToDest: args.sourceToDest,
	})
	if err != nil {
		return domain.FileEntry{}, fmt.Errorf("read/rewrite module file: %w", err)
	}

	return domain.FileEntry{Data: data, Mode: preserveMode(info.Mode())}, nil
}

func buildLock(in *domain.SyncInput, files []managedFile, gen []generatedRootTask) syncLock {
	var lock syncLock

	setLockSource(&lock, in)
	setLockConfiguration(&lock, in.Config)
	setLockResolvedModules(&lock, in)

	lock.GeneratedRootTasks = generatedRootTaskNames(gen)
	lock.ManagedFiles = files

	return lock
}

func buildPlanFromState(syncInput *domain.SyncInput, prev *previousState) (*domain.Plan, error) {
	artifacts, err := planAllFiles(syncInput, prev.lock)
	if err != nil {
		return nil, fmt.Errorf("plan all files: %w", err)
	}

	plan, meta := newPlanFromInputs(syncInput, &artifacts, prev)

	planResult, err := finalizeBuiltPlan(&finalizeBuiltPlanInput{
		syncInput: syncInput,
		plan:      plan,
		meta:      meta,
		artifacts: &artifacts,
	})
	if err != nil {
		return nil, fmt.Errorf("finalize plan plan: %w", err)
	}

	return planResult, nil
}

func finalizeBuiltPlan(input *finalizeBuiltPlanInput) (*domain.Plan, error) {
	planResult, err := finalizePlanDiff(&finalizePlanArgs{
		plan:         input.plan,
		workspace:    input.syncInput.Config.Workspace,
		rootBytes:    input.artifacts.rootBytes,
		rootState:    input.artifacts.rootState,
		syncRoot:     syncRootFromConfig(input.syncInput.Config),
		meta:         input.meta,
		metadataPath: config.MetadataPath(input.syncInput.Config),
	})
	if err != nil {
		return nil, fmt.Errorf("finalize plan diff: %w", err)
	}

	return planResult, nil
}

func buildRootPlanResult(input *buildRootPlanInput) (rootPlanResult, error) {
	finishInput, err := readRootPlanFinishInput(input)
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("read root plan finish input: %w", err)
	}

	rootResult, err := finishRootPlanResult(finishInput)
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("finish root plan result: %w", err)
	}

	return rootResult, nil
}

func readRootPlanFinishInput(input *buildRootPlanInput) (*finishRootPlanInput, error) {
	rootBytes, rootStateVal, err := readRootTaskfile(
		input.syncInput.TaskfileOps,
		input.syncInput.Config.Workspace,
		input.syncInput.Config.RootTaskfile,
	)
	if err != nil {
		return nil, fmt.Errorf("read root taskfile: %w", err)
	}

	return &finishRootPlanInput{
		syncInput:      input.syncInput,
		oldLock:        input.oldLock,
		moduleContents: input.moduleContents,
		rootBytes:      rootBytes,
		rootStateVal:   rootStateVal,
	}, nil
}

func buildRootTaskfile(args *buildRootArgs) (root []byte, tasks []generatedRootTask, err error) {
	inputs, err := prepareRootTaskfileInputs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare root taskfile inputs: %w", err)
	}

	rootBytes, generatedRootTasks, err := updateRootTaskfile(inputs)
	if err != nil {
		return nil, nil, fmt.Errorf("update root taskfile: %w", err)
	}

	return rootBytes, generatedRootTasks, nil
}

func rootTaskfileUpdateArgs(args *buildRootArgs, storeMetadata storeTaskMetaMap) *updateRootArgs {
	managedTasks, managedRootTasks := resolveManagedTasks(&planManagedInput{
		syncInput: args.syncInput, oldLock: args.oldLock,
	})
	moduleTaskfiles := collectModuleRootTaskfiles(&rootTaskfilesInput{
		requested: args.syncInput.Requested, moduleContents: args.moduleContents,
	})
	generatedRootTasks := buildGeneratedRootTasks(&groupModulesInput{
		requested: args.syncInput.Config.Tasks, requestedRecords: args.syncInput.Requested,
		metadata: storeMetadata, common: nil,
	})

	return &updateRootArgs{
		args:               *args,
		generatedRootTasks: generatedRootTasks,
		managedTasks:       managedTasks,
		managedRootTasks:   managedRootTasks,
		moduleTaskfiles:    moduleTaskfiles,
	}
}

func prepareRootTaskfileInputs(args *buildRootArgs) (*updateRootArgs, error) {
	storeMetadata, err := loadStoreTaskMetadata(args.syncInput.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("load store task metadata: %w", err)
	}

	return rootTaskfileUpdateArgs(args, storeMetadata), nil
}

func collectAndTrackModuleFiles(args *collectModuleArgs) (fMap, []managedFile, error) {
	contents, parentDocs, err := collectModuleContents(args)
	if err != nil {
		return nil, nil, fmt.Errorf("collect module contents: %w", err)
	}

	managed := appendManagedFiles(&appendManagedArgs{
		planned:    nil,
		mod:        args.mod,
		destDirRel: args.destDirRel,
		contents:   contents,
		parentDocs: parentDocs,
	})

	return contents, managed, nil
}

func collectModuleContents(args *collectModuleArgs) (fMap, map[string]struct{}, error) {
	policy := docPolicyFromConfig(args.syncInput.Config)

	contents, err := scanModuleFiles(moduleCollectOptions(args, policy))
	if err != nil {
		return nil, nil, fmt.Errorf("collect module files: %w", err)
	}

	parentDocs, err := mergeLogicalRootDocs(args, contents, policy)
	if err != nil {
		return nil, nil, fmt.Errorf("merge logical root docs: %w", err)
	}

	return contents, parentDocs, nil
}

func collectModuleFile(args *moduleCollectArgs) error {
	rel, err := relSlashPathWithOps(args.fsOps, args.sourceDir, args.absPath)
	if err != nil {
		return fmt.Errorf("rel slash path: %w", err)
	}

	if shouldSkipModuleFile(rel, args.docPolicy) {
		return nil
	}

	err = storeCollectedModuleFile(args, rel)
	if err != nil {
		return fmt.Errorf("store collected module file: %w", err)
	}

	return nil
}

func collectModuleRootTaskfiles(in *rootTaskfilesInput) map[string][]byte {
	moduleTaskfiles := make(map[string][]byte, len(in.requested))

	for task := range in.requested {
		rec := in.requested[task]
		files, ok := in.moduleContents[rec.SourceModule]

		if !ok {
			continue
		}

		if entry, hasRoot := files[rootTaskfileName]; hasRoot {
			moduleTaskfiles[task] = entry.Data
		}
	}

	return moduleTaskfiles
}

func collectModuleWalkFunc(
	opts *collectOptions,
	contents map[string]domain.FileEntry,
) fs.WalkDirFunc {
	return func(absPath string, entry os.DirEntry, walkErr error) error {
		return walkCollectModuleFile(&walkCollectArgs{
			opts:     opts,
			contents: contents,
			absPath:  absPath,
			entry:    entry,
			walkErr:  walkErr,
		})
	}
}

func walkCollectModuleFile(args *walkCollectArgs) error {
	if args.walkErr != nil {
		return fmt.Errorf(errWalk, args.absPath, args.walkErr)
	}

	if args.entry.IsDir() {
		return nil
	}

	err := collectWalkedModuleFile(args)
	if err != nil {
		return fmt.Errorf("walk collect module file: %w", err)
	}

	return nil
}

func collectWalkedModuleFile(args *walkCollectArgs) error {
	err := collectModuleFile(&moduleCollectArgs{
		ops:          args.opts.ops,
		sourceDir:    args.opts.sourceDir,
		fromDest:     args.opts.fromDest,
		absPath:      args.absPath,
		entry:        args.entry,
		docPolicy:    args.opts.docPolicy,
		sourceToDest: args.opts.sourceToDest,
		contents:     args.contents,
	})
	if err != nil {
		return fmt.Errorf("collect module file %q: %w", args.absPath, err)
	}

	return nil
}

func copyDocPathsInto(rootContents, contents fMap, parentDocs map[string]struct{}) {
	for rel := range rootContents {
		entry := rootContents[rel]

		if !pathutil.IsDocPath(rel) {
			continue
		}

		contents[rel] = entry
		parentDocs[rel] = struct{}{}
	}
}

func logicalRootReady(dir string) (bool, error) {
	return logicalRootReadyWithOps(defaultFileOps(), dir)
}

func logicalRootReadyWithOps(ops *fileOps, dir string) (bool, error) {
	info, err := withFileOps(ops).statPath(dir)
	iox.Discard(info)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("stat logical root %q: %w", dir, err)
}

func mergeLogicalRootDocs(
	args *collectModuleArgs,
	contents fMap,
	policy docPolicy,
) (map[string]struct{}, error) {
	parentDocs := make(map[string]struct{})

	if policy != docPolicyInclude {
		return parentDocs, nil
	}

	docs, err := mergeParentDocsIfDistinct(args, contents, parentDocs)
	if err != nil {
		return nil, fmt.Errorf("merge parent docs if distinct: %w", err)
	}

	return docs, nil
}

func mergeParentDocsIfDistinct(
	args *collectModuleArgs,
	contents fMap,
	parentDocs map[string]struct{},
) (map[string]struct{}, error) {
	mergeArgs := parentDocsMergeArgs(args, contents, parentDocs)

	if mergeArgs == nil {
		return parentDocs, nil
	}

	err := mergeParentDocFiles(mergeArgs)
	if err != nil {
		return nil, fmt.Errorf("merge parent doc files: %w", err)
	}

	return parentDocs, nil
}

func parentDocsMergeArgs(
	args *collectModuleArgs,
	contents fMap,
	parentDocs map[string]struct{},
) *mergeParentDocsArgs {
	destRoot := args.syncInput.Snapshot.ModuleDir(args.mod.DestinationModule)

	if sameModuleRoot(destRoot, args.sourceDir) {
		return nil
	}

	return &mergeParentDocsArgs{
		collect:    args,
		destRoot:   destRoot,
		contents:   contents,
		parentDocs: parentDocs,
		fsOps:      args.fsOps,
	}
}

func sameModuleRoot(destRoot, sourceDir string) bool {
	return filepath.Clean(destRoot) == filepath.Clean(sourceDir)
}

func mergeParentDocFiles(args *mergeParentDocsArgs) error {
	ready, err := logicalRootReadyWithOps(args.collect.fsOps, args.destRoot)
	if err != nil {
		return fmt.Errorf("logical root ready: %w", err)
	}

	if !ready {
		return nil
	}

	rootContents, err := scanLogicalRootDocs(args)
	if err != nil {
		return fmt.Errorf("scan logical root docs: %w", err)
	}

	copyDocPathsInto(rootContents, args.contents, args.parentDocs)

	return nil
}

func moduleCollectOptions(args *collectModuleArgs, policy docPolicy) *collectOptions {
	return &collectOptions{
		ops:          args.syncInput.TaskfileOps,
		fsOps:        args.fsOps,
		sourceDir:    args.sourceDir,
		fromDest:     args.mod.DestinationModule,
		docPolicy:    policy,
		sourceToDest: args.syncInput.SourceToDest,
	}
}

func scanLogicalRootDocs(args *mergeParentDocsArgs) (fMap, error) {
	rootContents, err := scanModuleFiles(&collectOptions{
		ops:          args.collect.syncInput.TaskfileOps,
		fsOps:        args.collect.fsOps,
		sourceDir:    args.destRoot,
		fromDest:     args.collect.mod.DestinationModule,
		docPolicy:    docPolicyInclude,
		sourceToDest: args.collect.syncInput.SourceToDest,
	})
	if err != nil {
		return nil, fmt.Errorf("scan logical root %q: %w", args.destRoot, err)
	}

	return rootContents, nil
}

func ensureSourceDirExists(sourceDir string, mod *moduleRecord) error {
	return ensureSourceDirExistsWithOps(defaultFileOps(), sourceDir, mod)
}

func ensureSourceDirExistsWithOps(ops *fileOps, sourceDir string, mod *moduleRecord) error {
	info, err := withFileOps(ops).statPath(sourceDir)
	iox.Discard(info)

	if err != nil {
		return domain.SyncError(
			fmt.Sprintf("source module directory %q does not exist", mod.SourceModule),
		)
	}

	return nil
}

func finalizePlanDiff(args *finalizePlanArgs) (*domain.Plan, error) {
	lists, err := diffPlan(args)
	if err != nil {
		return nil, fmt.Errorf("diff files: %w", err)
	}

	applyDiffResults(args.plan, &lists, &stagePathsInput{
		plan:         args.plan,
		workspace:    args.workspace,
		metadataPath: args.metadataPath,
		syncRoot:     args.syncRoot,
	})

	return args.plan, nil
}

func diffPlan(args *finalizePlanArgs) (diffLists, error) {
	return diffFiles(&diffInput{
		plan:         args.plan,
		workspace:    args.workspace,
		oldRoot:      oldRootForDiffing(args.rootBytes, args.rootState),
		syncRoot:     args.syncRoot,
		metadataPath: args.metadataPath,
		plannedMeta:  mustMarshalMetadata(args.meta),
		fsOps:        args.fsOps,
	})
}

func finishRootPlanResult(input *finishRootPlanInput) (rootPlanResult, error) {
	newRoot, generatedRootTasks, err := buildRootTaskfile(&buildRootArgs{
		syncInput:      input.syncInput,
		oldLock:        input.oldLock,
		moduleContents: input.moduleContents,
		rootBytes:      input.rootBytes,
	})
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("build root taskfile: %w", err)
	}

	return rootPlanResult{
		rootBytes:          input.rootBytes,
		rootState:          input.rootStateVal,
		newRoot:            newRoot,
		generatedRootTasks: generatedRootTasks,
	}, nil
}

func generatedRootTaskNames(generated []generatedRootTask) []string {
	names := make([]string, consts.IndexZero, len(generated))

	for i := range generated {
		names = append(names, generated[i].Name)
	}

	slices.Sort(names)

	return names
}

func isDestinationManaged(oldLock *syncLock, mod *moduleRecord) bool {
	if oldLock == nil {
		return false
	}

	for i := range oldLock.ManagedFiles {
		if oldLock.ManagedFiles[i].DestinationModule == mod.DestinationModule {
			return true
		}
	}

	return false
}

func mustMarshalMetadata(meta *domain.Metadata) []byte {
	return MarshalMetadata(meta)
}

func newPlanFromInputs(
	syncIn *domain.SyncInput,
	art *planArtifacts,
	prev *previousState,
) (plan *domain.Plan, meta *domain.Metadata) {
	lock := buildLock(syncIn, art.plannedFiles, art.generatedRootTasks)

	meta = newPlanMetadata(syncIn)

	plan = assemblePlan(&assemblePlanInput{
		syncInput: syncIn,
		artifacts: art,
		prev:      *prev,
		lock:      lock,
		meta:      meta,
	})

	return plan, meta
}

func newPlanMetadata(syncInput *domain.SyncInput) *domain.Metadata {
	return &domain.Metadata{
		TargetFolder:      syncInput.Config.TargetFolder,
		LockFile:          config.LockFilePath(syncInput.Config),
		ConfigurationHash: syncInput.Config.ConfigurationHash,
	}
}

func oldRootForDiffing(rootBytes []byte, state rootState) []byte {
	if state == rootAbsent {
		return nil
	}

	return rootBytes
}

func artifactsFromRootPlan(files []managedFile, mc mcMap, root *rootPlanResult) planArtifacts {
	return planArtifacts{
		plannedFiles:       files,
		moduleContents:     mc,
		rootBytes:          root.rootBytes,
		rootState:          root.rootState,
		newRoot:            root.newRoot,
		generatedRootTasks: root.generatedRootTasks,
	}
}

func planAllFiles(syncInput *domain.SyncInput, oldLock *syncLock) (planArtifacts, error) {
	plannedFiles, moduleContents, err := planManagedFiles(&planManagedInput{
		syncInput: syncInput, oldLock: oldLock,
	})
	if err != nil {
		return planArtifacts{}, fmt.Errorf("plan managed files: %w", err)
	}

	rootResult, err := planRootTaskfile(&buildRootPlanInput{
		syncInput: syncInput, oldLock: oldLock, moduleContents: moduleContents,
	})
	if err != nil {
		return planArtifacts{}, fmt.Errorf("plan root taskfile: %w", err)
	}

	return artifactsFromRootPlan(plannedFiles, moduleContents, &rootResult), nil
}

func planAllModules(allModules []moduleRecord, args *modulePlanArgs) ([]managedFile, error) {
	var planned []managedFile

	for i := range allModules {
		mod := &allModules[i]

		var err error

		planned, err = accumulateModulePlan(&modulePlanArgs{
			syncInput:      args.syncInput,
			mod:            mod,
			oldLock:        args.oldLock,
			moduleContents: args.moduleContents,
			planned:        planned,
		})
		if err != nil {
			return nil, fmt.Errorf("accumulate module plan for %q: %w", mod.SourceModule, err)
		}
	}

	return planned, nil
}

func planManagedFiles(args *planManagedInput) ([]managedFile, mcMap, error) {
	allModules := sortedModules(args.syncInput)
	moduleContents := make(mcMap)

	planned, err := planAllModules(allModules, &modulePlanArgs{
		syncInput:      args.syncInput,
		mod:            nil,
		oldLock:        args.oldLock,
		moduleContents: moduleContents,
		planned:        nil,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("plan all modules: %w", err)
	}

	sortManagedFiles(planned)

	return planned, moduleContents, nil
}

func planModuleFiles(args *modulePlanArgs) (fMap, []managedFile, error) {
	collectArgs, err := moduleCollectArgsForPlan(args)
	if err != nil {
		return nil, nil, fmt.Errorf("module collect args for plan: %w", err)
	}

	contents, managed, err := collectAndTrackModuleFiles(collectArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("collect and track module files: %w", err)
	}

	return contents, managed, nil
}

func moduleCollectArgsForPlan(args *modulePlanArgs) (*collectModuleArgs, error) {
	sourceDir := args.syncInput.Snapshot.ModuleDir(args.mod.SourceModule)

	destDirRel, err := prepareModulePlanDirs(&modulePlanDirsInput{
		syncInput: args.syncInput,
		mod:       args.mod,
		oldLock:   args.oldLock,
		sourceDir: sourceDir,
		fsOps:     args.fsOps,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare module plan dirs: %w", err)
	}

	return &collectModuleArgs{
		syncInput: args.syncInput, mod: args.mod, sourceDir: sourceDir, destDirRel: destDirRel,
		fsOps: args.fsOps,
	}, nil
}

func prepareModulePlanDirs(input *modulePlanDirsInput) (string, error) {
	err := ensureSourceDirExistsWithOps(input.fsOps, input.sourceDir, input.mod)
	if err != nil {
		return consts.Empty, fmt.Errorf("ensure source dir exists: %w", err)
	}

	destDirRel, err := validateModuleDestination(&modulePlanArgs{
		syncInput: input.syncInput, mod: input.mod, oldLock: input.oldLock,
		moduleContents: nil, planned: nil,
		fsOps: input.fsOps,
	})
	if err != nil {
		return consts.Empty, fmt.Errorf("validate module destination: %w", err)
	}

	return destDirRel, nil
}

func planRootTaskfile(input *buildRootPlanInput) (rootPlanResult, error) {
	if !input.syncInput.Config.SyncRoot {
		return emptyRootPlanResult(), nil
	}

	rootResult, err := buildRootPlanResult(input)
	if err != nil {
		return emptyRootPlanResult(), fmt.Errorf("build root plan result: %w", err)
	}

	return rootResult, nil
}

func readAndMaybeRewriteModuleFile(args *rewriteModuleArgs) ([]byte, error) {
	data, err := pathutil.ReadRelativeFile(args.sourceDir, args.rel)
	if err != nil {
		return nil, fmt.Errorf("read module file %q: %w", args.absPath, err)
	}

	rewritten, err := maybeRewriteRootTaskfile(args, data)
	if err != nil {
		return nil, fmt.Errorf("rewrite module file %q: %w", args.absPath, err)
	}

	return rewritten, nil
}

func maybeRewriteRootTaskfile(args *rewriteModuleArgs, data []byte) ([]byte, error) {
	if args.rel != rootTaskfileName || args.ops == nil {
		return data, nil
	}

	rewritten, err := args.ops.RewriteIncludes(data, args.sourceToDest, args.fromDest)
	if err != nil {
		return nil, fmt.Errorf("rewrite includes in %q: %w", args.absPath, err)
	}

	return rewritten, nil
}

func readRootTaskfile(
	ops ports.TaskfileOps,
	workspace, rootPath string,
) ([]byte, rootState, error) {
	rootBytes, err := pathutil.ReadRelativeFile(workspace, rootPath)
	if err == nil {
		return rootBytes, rootPresent, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, rootAbsent, fmt.Errorf("read root Taskfile.yml: %w", err)
	}

	template, templateErr := rootTemplateOrError(ops)
	if templateErr != nil {
		return nil, rootAbsent, fmt.Errorf("root taskfile template: %w", templateErr)
	}

	return template, rootAbsent, nil
}

func rootTemplateOrError(ops ports.TaskfileOps) ([]byte, error) {
	if ops == nil {
		return nil, errTaskfileOpsNotConfigured
	}

	return ops.NewRootTemplate(), nil
}

func relSlashPath(sourceDir, absPath string) (string, error) {
	return relSlashPathWithOps(defaultFileOps(), sourceDir, absPath)
}

func relSlashPathWithOps(ops *fileOps, sourceDir, absPath string) (string, error) {
	return relToSlashWithOps(ops, sourceDir, absPath, "rel path")
}

func resolveManagedTasks(args *planManagedInput) (managedTasks, managedRootTasks []string) {
	if args.oldLock != nil {
		return args.oldLock.Configuration.Tasks, args.oldLock.GeneratedRootTasks
	}

	return args.syncInput.Config.Tasks, nil
}

func scanModuleFiles(opts *collectOptions) (map[string]domain.FileEntry, error) {
	opts.fsOps = withFileOps(opts.fsOps)

	contents := make(map[string]domain.FileEntry)

	err := opts.fsOps.walkDir(
		opts.sourceDir,
		collectModuleWalkFunc(opts, contents),
	)
	if err != nil {
		return nil, fmt.Errorf("walk module directory %q: %w", opts.sourceDir, err)
	}

	return contents, nil
}

func setLockConfiguration(lock *syncLock, cfg *config.Config) {
	lock.Configuration.TargetFolder = cfg.TargetFolder
	lock.Configuration.Tasks = append([]string{}, cfg.Tasks...)
	lock.Configuration.NodePackageManager = cfg.NodePackageManager
	lock.Configuration.IncludesDoc = cfg.IncludesDoc
	lock.Configuration.SyncRoot = cfg.SyncRoot
}

func setLockResolvedModules(lock *syncLock, syncInput *domain.SyncInput) {
	lock.Requested = syncInput.Requested
	lock.Dependencies = append([]moduleRecord{}, syncInput.Dependencies...)
}

func setLockSource(lock *syncLock, syncInput *domain.SyncInput) {
	lock.Source.Repository = config.StoreRepository
	lock.Source.RequestedVersion = syncInput.Config.StoreVersion
	lock.Source.SourceRef = syncInput.Snapshot.SourceRef()
	lock.Source.ResolvedCommit = syncInput.Snapshot.ResolvedCommit()
	lock.Source.DefaultBranch = syncInput.Snapshot.DefaultBranch()
}

func shouldSkipModuleFile(rel string, policy docPolicy) bool {
	if policy == docPolicySkip && pathutil.IsDocPath(rel) {
		return true
	}

	return pathutil.IsTestPath(rel) || pathutil.IsModuleMetadataPath(rel)
}

func sortManagedFiles(planned []managedFile) {
	slices.SortFunc(planned, func(a, b managedFile) int {
		if a.Path == b.Path {
			return strings.Compare(a.SourceModule, b.SourceModule)
		}

		return strings.Compare(a.Path, b.Path)
	})
}

// sortedModules merges the requested and dependency modules into a single
// source-module-ordered slice so planning is deterministic.
func sortedModules(syncInput *domain.SyncInput) []moduleRecord {
	allModules := make(
		[]moduleRecord,
		consts.IndexZero,
		len(syncInput.Requested)+len(syncInput.Dependencies),
	)

	for key := range syncInput.Requested {
		allModules = append(allModules, syncInput.Requested[key])
	}

	allModules = append(allModules, syncInput.Dependencies...)
	slices.SortFunc(allModules, func(a, b moduleRecord) int {
		return strings.Compare(a.SourceModule, b.SourceModule)
	})

	return allModules
}

func storeCollectedModuleFile(args *moduleCollectArgs, rel string) error {
	entry, err := buildFileEntry(&fileEntryArgs{
		ops:          args.ops,
		sourceDir:    args.sourceDir,
		fromDest:     args.fromDest,
		rel:          rel,
		absPath:      args.absPath,
		entry:        args.entry,
		sourceToDest: args.sourceToDest,
	})
	if err != nil {
		return fmt.Errorf("build file entry: %w", err)
	}

	args.contents[rel] = entry

	return nil
}

func unmanagedDestinationError(mod *moduleRecord) domain.SyncError {
	return domain.SyncError(fmt.Sprintf(
		`Cannot copy source module %q to %q: the destination exists but is not managed by TaskOtter.`,
		mod.SourceModule,
		mod.Path,
	))
}

func updateRootTaskfile(input *updateRootArgs) (root []byte, tasks []generatedRootTask, err error) {
	ops := input.args.syncInput.TaskfileOps

	if ops == nil {
		return nil, nil, errTaskfileOpsNotConfigured
	}

	root, err = ops.UpdateRootTaskfile(input.args.rootBytes, rootUpdateInputFrom(input))
	if err != nil {
		return nil, nil, fmt.Errorf("update root Taskfile.yml: %w", err)
	}

	return root, input.generatedRootTasks, nil
}

func rootUpdateInputFrom(input *updateRootArgs) *rootUpdateInput {
	cfg := input.args.syncInput.Config

	return &rootUpdateInput{
		Tasks:            cfg.Tasks,
		TargetFolder:     cfg.TargetFolder,
		RootTaskfileDir:  path.Dir(cfg.RootTaskfile),
		DestByTask:       input.args.syncInput.DestByTask,
		ManagedTasks:     input.managedTasks,
		ModuleTaskfiles:  input.moduleTaskfiles,
		GeneratedTasks:   input.generatedRootTasks,
		ManagedRootTasks: input.managedRootTasks,
	}
}

func validateDestination(destDirAbs string, mod *moduleRecord, oldLock *syncLock) error {
	return validateDestinationWithOps(defaultFileOps(), destDirAbs, mod, oldLock)
}

func validateDestinationWithOps(
	ops *fileOps,
	destDirAbs string,
	mod *moduleRecord,
	oldLock *syncLock,
) error {
	info, err := withFileOps(ops).statPath(destDirAbs)

	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("stat destination %q: %w", mod.Path, err)
	}

	err = validateExistingDestination(info, mod, oldLock)
	if err != nil {
		return fmt.Errorf("validate existing destination: %w", err)
	}

	return nil
}

func validateExistingDestination(info os.FileInfo, mod *moduleRecord, oldLock *syncLock) error {
	if !info.IsDir() {
		return domain.SyncError(
			fmt.Sprintf("destination %q exists and is not a directory", mod.Path),
		)
	}

	if isDestinationManaged(oldLock, mod) {
		return nil
	}

	return unmanagedDestinationError(mod)
}

func validateModuleDestination(args *modulePlanArgs) (string, error) {
	destDirRel := pathutil.JoinRelative(
		args.syncInput.Config.TargetFolder,
		args.mod.DestinationModule,
	)
	destDirAbs := pathutil.WorkspacePath(args.syncInput.Config.Workspace, destDirRel)

	err := validateDestinationWithOps(args.fsOps, destDirAbs, args.mod, args.oldLock)
	if err != nil {
		return consts.Empty, fmt.Errorf("validate destination: %w", err)
	}

	return destDirRel, nil
}

// PrepareSyncInput maps resolved modules and dependencies into syncer input records.
func PrepareSyncInput(args *PrepareSyncInputArgs) (domain.SyncInput, error) {
	requestedSources := collectRequestedSources(args.Resolutions)
	allSources := append(append([]string{}, requestedSources...), args.DepSources...)

	input, err := assembleSyncInput(args, allSources)
	if err != nil {
		return domain.SyncInput{}, fmt.Errorf("assemble sync input: %w", err)
	}

	return input, nil
}

func assembleSyncInput(args *PrepareSyncInputArgs, allSources []string) (domain.SyncInput, error) {
	sourceToDest, err := resolvesvc.BuildDestinationMap(allSources)
	if err != nil {
		return domain.SyncInput{}, fmt.Errorf("build destination map: %w", err)
	}

	requestedRecords, destByTask := buildReqRecords(
		&buildReqArgs{cfg: args.Cfg, res: args.Resolutions, src: sourceToDest},
	)
	dependencyRecords := buildDepRecords(args.Cfg, args.DepSources, sourceToDest)

	return domain.SyncInput{
		Config:       args.Cfg,
		Snapshot:     args.Snapshot,
		TaskfileOps:  args.TaskfileOps,
		Requested:    requestedRecords,
		Dependencies: dependencyRecords,
		SourceToDest: sourceToDest,
		DestByTask:   destByTask,
	}, nil
}

func buildDepRecords(cfg *config.Config, deps []string, src map[string]string) []modRec {
	dependencyRecords := make([]modRec, consts.IndexZero, len(deps))

	for i := range deps {
		dep := deps[i]
		dest := src[dep]

		dependencyRecords = append(dependencyRecords, modRec{
			SourceModule:      dep,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(cfg.TargetFolder, dest),
		})
	}

	return dependencyRecords
}

func buildReqRecords(args *buildReqArgs) (recMap, map[string]string) {
	reqRecs := make(recMap)
	dstByTask := make(map[string]string)

	for i := range args.res {
		item := &args.res[i]
		dest := args.src[item.SourceModule]

		reqRecs[item.LogicalTask] = modRec{
			SourceModule:      item.SourceModule,
			DestinationModule: dest,
			Path:              pathutil.JoinRelative(args.cfg.TargetFolder, dest),
		}
		dstByTask[item.LogicalTask] = dest
	}

	return reqRecs, dstByTask
}

func collectRequestedSources(resolutions []resolvesvc.Resolution) []string {
	requestedSources := make([]string, consts.IndexZero, len(resolutions))

	for i := range resolutions {
		requestedSources = append(requestedSources, resolutions[i].SourceModule)
	}

	return requestedSources
}

// DefaultBranch implements ports.Snapshot.
func (port *SnapshotAdapter) DefaultBranch() string {
	return storedomain.DefaultBranch(port.snap)
}

// ModuleDir implements ports.Snapshot.
func (port *SnapshotAdapter) ModuleDir(sourceModule string) string {
	return storedomain.ModuleDir(port.snap, sourceModule)
}

// ResolvedCommit implements ports.Snapshot.
func (port *SnapshotAdapter) ResolvedCommit() string {
	return storedomain.ResolvedCommit(port.snap)
}

// SourceRef implements ports.Snapshot.
func (port *SnapshotAdapter) SourceRef() string {
	return storedomain.SourceRef(port.snap)
}

// WorkspaceRoot implements ports.Snapshot.
func (port *SnapshotAdapter) WorkspaceRoot() string {
	return storedomain.WorkspaceRoot(port.snap)
}

// SnapshotPort adapts snap for PrepareSyncInput and related sync ports.
func SnapshotPort(snap *storedomain.Snapshot) *SnapshotAdapter {
	return &SnapshotAdapter{snap: snap}
}

func appendUniqueExportedTask(out []string, seen map[string]struct{}, task string) []string {
	if task == consts.Empty {
		return out
	}

	if _, ok := seen[task]; ok {
		return out
	}

	seen[task] = struct{}{}

	return append(out, task)
}

func climbToParentModule(current string) (string, bool) {
	parent := path.Dir(current)

	if parent == "." || parent == current {
		return consts.Empty, false
	}

	return parent, true
}

func commonStoreTaskNames(metadata map[string]storeTaskMetadata) map[string]struct{} {
	counts := countExportedTasksPerModule(metadata)
	common := make(map[string]struct{})

	for task := range counts {
		if counts[task] >= domain.YAMLMappingPairKeyValue {
			common[task] = struct{}{}
		}
	}

	return common
}

func countExportedTasksPerModule(metadata map[string]storeTaskMetadata) map[string]int {
	counts := make(map[string]int)

	for key := range metadata {
		meta := metadata[key]
		tasks := dedupExportedTasks(meta.ExportedTasks)

		for i := range tasks {
			counts[tasks[i]]++
		}
	}

	return counts
}

func dedupExportedTasks(exportedTasks []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, consts.IndexZero, len(exportedTasks))

	for i := range exportedTasks {
		out = appendUniqueExportedTask(out, seen, exportedTasks[i])
	}

	return out
}

func emptyStoreTaskMetadata() storeTaskMetadata {
	return storeTaskMetadata{
		Schema:        consts.Empty,
		Module:        consts.Empty,
		Taskfile:      consts.Empty,
		ExportedTasks: nil,
		Variants:      nil,
	}
}

func loadOneStoreMetadataFile(root, abs string, out map[string]storeTaskMetadata) error {
	return loadOneStoreMetadataFileWithOps(defaultFileOps(), root, abs, out)
}

func loadOneStoreMetadataFileWithOps(
	ops *fileOps,
	root, abs string,
	out map[string]storeTaskMetadata,
) error {
	module, err := moduleNameForWithOps(ops, root, abs)
	if err != nil {
		return fmt.Errorf("module name for %q: %w", abs, err)
	}

	meta, err := readStoreTaskMetadata(abs, module)
	if err != nil {
		return fmt.Errorf("read store task metadata %q: %w", abs, err)
	}

	out[module] = meta

	return nil
}

func loadStoreTaskMetadata(snapshot ports.Snapshot) (map[string]storeTaskMetadata, error) {
	return loadStoreTaskMetadataWithOps(defaultFileOps(), snapshot)
}

func loadStoreTaskMetadataWithOps(
	ops *fileOps,
	snapshot ports.Snapshot,
) (map[string]storeTaskMetadata, error) {
	out := make(map[string]storeTaskMetadata)
	root := filepath.Join(snapshot.WorkspaceRoot(), taskfilesDirName)

	err := withFileOps(ops).walkDir(root, storeMetadataWalker(root, out))
	if err != nil {
		return nil, fmt.Errorf("load store metadata: %w", err)
	}

	return out, nil
}

func moduleNameFor(root, abs string) (string, error) {
	return moduleNameForWithOps(defaultFileOps(), root, abs)
}

func moduleNameForWithOps(ops *fileOps, root, abs string) (string, error) {
	relDir, err := withFileOps(ops).relPath(root, filepath.Dir(abs))
	if err != nil {
		return consts.Empty, fmt.Errorf("metadata module path for %q: %w", abs, err)
	}

	return filepath.ToSlash(relDir), nil
}

func readStoreTaskMetadata(abs, module string) (storeTaskMetadata, error) {
	data, err := pathutil.ReadRelativeFile(filepath.Dir(abs), storeMetadataFileName)
	if err != nil {
		return emptyStoreTaskMetadata(), fmt.Errorf("read metadata for %q: %w", module, err)
	}

	meta, err := parseStoreTaskMetadata(data, module)
	if err != nil {
		return emptyStoreTaskMetadata(), fmt.Errorf("metadata for %q: %w", module, err)
	}

	return meta, nil
}

func parseStoreTaskMetadata(data []byte, module string) (storeTaskMetadata, error) {
	var meta storeTaskMetadata

	err := yaml.Unmarshal(data, &meta)
	if err != nil {
		return emptyStoreTaskMetadata(), fmt.Errorf("parse metadata: %w", err)
	}

	if meta.Schema != storeMetadataSchema {
		return emptyStoreTaskMetadata(), fmt.Errorf(
			"%w %q in %q: expected %q",
			errUnsupportedStoreMetadataSchema, meta.Schema, module, storeMetadataSchema,
		)
	}

	if meta.Module == consts.Empty {
		meta.Module = module
	}

	return meta, nil
}

func resolveStoreTaskMetadata(src string, meta storeTaskMetaMap) (storeTaskMetadata, bool) {
	current := src

	for current != consts.Empty {
		if meta, foundMeta := meta[current]; foundMeta {
			return meta, true
		}

		parent, ok := climbToParentModule(current)

		if !ok {
			break
		}

		current = parent
	}

	return emptyStoreTaskMetadata(), false
}

func storeMetadataWalker(root string, out map[string]storeTaskMetadata) fs.WalkDirFunc {
	return func(abs string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf(errWalk, abs, walkErr)
		}

		if entry.IsDir() || entry.Name() != storeMetadataFileName {
			return nil
		}

		return loadOneStoreMetadataFile(root, abs, out)
	}
}

func addCommonExportedTasks(input *addCommonTasksInput) {
	for i := range input.exportedTasks {
		exported := input.exportedTasks[i]

		if _, ok := input.common[exported]; !ok {
			continue
		}

		input.modulesByTask[exported] = append(input.modulesByTask[exported], input.logicalTask)
	}
}

func buildGenRootTaskList(names []string, byTask map[string][]string) []genRootTask {
	generated := make([]genRootTask, consts.IndexZero, len(names))

	for i := range names {
		name := names[i]

		generated = append(generated, genRootTask{
			Name:    name,
			Modules: byTask[name],
		})
	}

	return generated
}

func buildGeneratedRootTasks(input *groupModulesInput) []genRootTask {
	input.common = commonStoreTaskNames(input.metadata)

	return finalizeGeneratedTasks(groupModulesByGeneratedTask(input))
}

func finalizeGeneratedTasks(modulesByTask map[string][]string) []genRootTask {
	names := qualifyingGeneratedTaskNames(modulesByTask)
	slices.Sort(names)

	return buildGenRootTaskList(names, modulesByTask)
}

func generatedTaskMetadata(input *groupModulesInput, logicalTask string) (storeTaskMetadata, bool) {
	record, foundRecord := input.requestedRecords[logicalTask]

	if !foundRecord {
		return storeTaskMetadata{
			Schema:        consts.Empty,
			Module:        consts.Empty,
			Taskfile:      consts.Empty,
			ExportedTasks: nil,
			Variants:      nil,
		}, false
	}

	return resolveStoreTaskMetadata(record.SourceModule, input.metadata)
}

func groupModulesByGeneratedTask(input *groupModulesInput) map[string][]string {
	modulesByTask := make(map[string][]string)

	for i := range input.requested {
		recordGeneratedTaskModules(modulesByTask, input, input.requested[i])
	}

	return modulesByTask
}

func qualifyingGeneratedTaskNames(modulesByTask map[string][]string) []string {
	names := make([]string, consts.IndexZero, len(modulesByTask))

	for name := range modulesByTask {
		if len(modulesByTask[name]) >= consts.IndexTwo {
			names = append(names, name)
		}
	}

	return names
}

func recordGeneratedTaskModules(byTask map[string][]string, input *groupModulesInput, task string) {
	meta, ok := generatedTaskMetadata(input, task)

	if !ok {
		return
	}

	addCommonExportedTasks(&addCommonTasksInput{
		modulesByTask: byTask,
		exportedTasks: meta.ExportedTasks,
		common:        input.common,
		logicalTask:   task,
	})
}

func collectOptionsFrom(opts *CollectOptions) *collectOptions {
	return &collectOptions{
		ops:          opts.TaskfileOps,
		fsOps:        opts.fsOps,
		sourceDir:    opts.SourceDir,
		fromDest:     opts.FromDest,
		docPolicy:    docPolicyFromExported(opts.DocPolicy),
		sourceToDest: opts.SourceToDest,
	}
}

func docPolicyFromConfig(cfg *config.Config) docPolicy {
	if cfg.IncludesDoc {
		return docPolicyInclude
	}

	return docPolicySkip
}

func docPolicyFromExported(policy DocPolicy) docPolicy {
	if policy == DocPolicyInclude {
		return docPolicyInclude
	}

	return docPolicySkip
}

func syncRootFromConfig(cfg *config.Config) syncRootPolicy {
	if cfg.SyncRoot {
		return syncRootEnabled
	}

	return syncRootDisabled
}

// MarshalLock encodes a lock file using stable on-disk keys.
func MarshalLock(lock *syncLock) []byte {
	return lockmodel.MarshalLock(lock)
}

// MarshalMetadata encodes metadata using stable on-disk keys.
func MarshalMetadata(meta *domain.Metadata) []byte {
	return domain.MarshalMetadata(meta)
}
