// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package cli provides helpers for running git commands used by the sync workflow.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
)

type (
	pathSet = map[string]struct{}

	// BranchChecker reads branch metadata used for ownership checks.
	BranchChecker interface {
		BranchExists(ctx context.Context, branch string) (bool, error)
		LastCommitMessage(ctx context.Context, branch string) (string, error)
	}

	// Brancher manages local branch refs.
	Brancher interface {
		CheckoutBranch(ctx context.Context, branch string) error
		CreateOrResetBranch(ctx context.Context, branch string) error
		BranchExists(ctx context.Context, branch string) (bool, error)
		LastCommitMessage(ctx context.Context, branch string) (string, error)
		DefaultBranch(ctx context.Context) (string, error)
	}

	// Indexer inspects and stages the working tree.
	Indexer interface {
		HasUnrelatedChanges(ctx context.Context, set map[string]struct{}) (bool, error)
		Stage(ctx context.Context, paths []string) error
		Commit(ctx context.Context, message string) error
	}

	// Publisher pushes branches to origin.
	Publisher interface {
		Push(ctx context.Context, branch string) error
		PushForceWithLease(ctx context.Context, branch string) error
	}

	// Client runs git commands in a workspace directory.
	Client struct {
		workspace string
	}
)

const (

	// SyncCommitMessage is the commit message TaskOtter uses for sync branches.
	SyncCommitMessage = "chore(taskotter): sync taskfiles"

	commitUserName = "TaskOtter"

	commitUserEmail = "taskotter@users.noreply.github.com"

	gitStatusPathOffset = 3

	gitRemote = "remote"

	gitVerifyFlag = "--verify"

	gitConfigFlag = "-c"

	gitBinary = "git"

	fmtGitCmdErr = "git %s: %w: %s"

	argSep = " "

	errMustNotStartWithHyphen = "%w: must not start with '-'"

	fmtValidateGitRefErr = "validate git ref: %w"

	gitCheckout = "checkout"

	fmtCheckoutBranchErr = "checkout branch: %w"

	gitPush = "push"

	fmtGitPushErr = "git push: %w"

	maxGitRefLen = 255
)

var (
	errOriginHEADNotAvailable = errors.New("origin HEAD not available")

	errNoRemoteBranchAtOriginHEAD = errors.New("no remote branch at origin HEAD commit")

	errHEADBranchNotFound = errors.New("HEAD branch not found in remote show output")

	errDefaultBranchDetectionFailed = errors.New(
		"detect default branch: none of the detection methods succeeded",
	)

	errBranchNotOwned = errors.New("branch exists but is not owned by TaskOtter")

	errInvalidGitRef = errors.New("invalid git ref")

	errInvalidStagePath = errors.New("invalid stage path")

	gitRefPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
)

// NewClient returns a git client bound to the given workspace path.
func NewClient(workspace string) *Client {
	return &Client{workspace: workspace}
}

// AllowedPathSet converts staged path strings into a lookup set.
func AllowedPathSet(paths []string) map[string]struct{} {
	out := make(map[string]struct{}, len(paths))

	for i := range paths {
		out[filepath.ToSlash(paths[i])] = struct{}{}
	}

	return out
}

// EnsureBranchOwned allows new sync branches and rejects foreign branch reuse.
func EnsureBranchOwned(ctx context.Context, ops BranchChecker, branch string) error {
	exists, err := ops.BranchExists(ctx, branch)
	if err != nil {
		return fmt.Errorf("check branch exists: %w", err)
	}

	if !exists {
		return nil
	}

	err = verifyExistingBranchOwned(ctx, ops, branch)
	if err != nil {
		return fmt.Errorf("verify existing branch owned: %w", err)
	}

	return nil
}

// IsGitRepo reports whether workspace contains a .git directory.
func IsGitRepo(workspace string) bool {
	info, err := os.Stat(filepath.Join(workspace, ".git"))
	iox.Discard(info)

	return err == nil
}

// ValidateGitRef checks that ref is safe to pass to git commands.
func ValidateGitRef(ref string) error {
	ref = strings.TrimSpace(ref)

	if ref == consts.Empty {
		return fmt.Errorf("%w: must not be empty", errInvalidGitRef)
	}

	if strings.HasPrefix(ref, consts.Hyphen) {
		return fmt.Errorf(errMustNotStartWithHyphen, errInvalidGitRef)
	}

	if len(ref) > maxGitRefLen {
		return fmt.Errorf("%w: exceeds maximum length", errInvalidGitRef)
	}

	if !gitRefPattern.MatchString(ref) {
		return fmt.Errorf("%w: %q contains invalid characters", errInvalidGitRef, ref)
	}

	return nil
}

// ValidateStagePath checks that path is a safe workspace-relative git add target.
func ValidateStagePath(workspace, path string) error {
	trimmed := strings.TrimSpace(path)

	if strings.HasPrefix(trimmed, consts.Hyphen) {
		return fmt.Errorf(errMustNotStartWithHyphen, errInvalidStagePath)
	}

	validated, err := pathutil.ValidateRelativePath(workspace, path)
	iox.Discard(validated)

	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidStagePath, err)
	}

	return nil
}

// WriteLocalIdentity configures commit author metadata for sync commits.
func WriteLocalIdentity() {
	// Commit identity is applied per command via -c; config files are not writable
	// in GitHub Actions Docker containers.
}

func checkBranchOwnership(msg, branch string) error {
	if msg != SyncCommitMessage {
		return fmt.Errorf("%w: %q", errBranchNotOwned, branch)
	}

	return nil
}

func defaultBranchFailure(refreshErr error) error {
	if refreshErr != nil {
		return errors.Join(
			errDefaultBranchDetectionFailed,
			fmt.Errorf("refresh origin head: %w", refreshErr),
		)
	}

	return errDefaultBranchDetectionFailed
}

func firstPlausibleRef(refs string) (string, bool) {
	for line := range strings.SplitSeq(strings.TrimSpace(refs), consts.Newline) {
		branch, ok := plausibleBranchFromLine(line)

		if ok {
			return branch, true
		}
	}

	return consts.Empty, false
}

func firstPlausibleRefOrError(refs string) (string, error) {
	branch, ok := firstPlausibleRef(refs)

	if !ok {
		return consts.Empty, errNoRemoteBranchAtOriginHEAD
	}

	return branch, nil
}

func hasUnrelatedStatusLines(out string, allowed pathSet) bool {
	for line := range strings.SplitSeq(strings.TrimSpace(out), consts.Newline) {
		if isUnrelatedStatusLine(line, allowed) {
			return true
		}
	}

	return false
}

func isAllowedPath(path string, allowed pathSet) bool {
	for allowedPath := range allowed {
		if path == allowedPath || strings.HasPrefix(path, allowedPath+"/") {
			return true
		}
	}

	return false
}

func isChangeAllowed(path string, allowed pathSet) bool {
	if _, ok := allowed[path]; ok {
		return true
	}

	return isAllowedPath(path, allowed)
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isHexDigit(r rune) bool {
	return isDigit(r) || isLowerHexLetter(r) || isUpperHexLetter(r)
}

func isHexString(value string) bool {
	for i := range len(value) {
		if !isHexDigit(rune(value[i])) {
			return false
		}
	}

	return true
}

func isLikelyCommitSHA(branch string) bool {
	return len(branch) >= consts.IndexSeven && isHexString(branch)
}

func isLowerHexLetter(r rune) bool {
	return r >= 'a' && r <= 'f'
}

func isPlausibleDefaultBranch(branch string) bool {
	branch = strings.TrimSpace(branch)

	if isReservedBranchName(branch) {
		return false
	}

	if isLikelyCommitSHA(branch) {
		return false
	}

	return true
}

func isReservedBranchName(branch string) bool {
	return branch == consts.Empty || branch == "HEAD" || branch == consts.GitOrigin
}

func isUnrelatedStatusLine(line string, allowed pathSet) bool {
	path, ok := parseStatusPath(line)

	return ok && !isChangeAllowed(path, allowed)
}

func isUpperHexLetter(r rune) bool {
	return r >= 'A' && r <= 'F'
}

func normalizeBranch(name string) string {
	branch := strings.TrimSpace(name)

	return strings.TrimPrefix(branch, "origin/")
}

func parseHEADBranchLine(out string) (string, bool) {
	const prefix = "HEAD branch: "

	for line := range strings.SplitSeq(out, consts.Newline) {
		line = strings.TrimSpace(line)

		after, ok := strings.CutPrefix(line, prefix)

		if !ok {
			continue
		}

		branch := strings.TrimSpace(after)

		if branch != consts.Empty {
			return branch, true
		}
	}

	return consts.Empty, false
}

func parseStatusPath(line string) (string, bool) {
	if line == consts.Empty || len(line) < gitStatusPathOffset {
		return consts.Empty, false
	}

	path := strings.TrimSpace(line[gitStatusPathOffset:])

	if path == consts.Empty {
		return consts.Empty, false
	}

	return path, true
}

func plausibleBranchFromLine(line string) (string, bool) {
	line = strings.TrimSpace(line)

	if line == consts.Empty || line == "origin/HEAD" {
		return consts.Empty, false
	}

	branch := normalizeBranch(line)

	return branch, isPlausibleDefaultBranch(branch)
}

func validateStagePaths(workspace string, paths []string) error {
	for i := range paths {
		err := ValidateStagePath(workspace, paths[i])
		if err != nil {
			return fmt.Errorf("validate stage path: %w", err)
		}
	}

	return nil
}

func verifyExistingBranchOwned(ctx context.Context, ops BranchChecker, branch string) error {
	msg, err := ops.LastCommitMessage(ctx, branch)
	if err != nil {
		return fmt.Errorf("read last commit message: %w", err)
	}

	err = checkBranchOwnership(msg, branch)
	if err != nil {
		return fmt.Errorf("check branch ownership: %w", err)
	}

	return nil
}

func branchRefVerified(ctx context.Context, client *Client, ref string) bool {
	out, err := client.output(ctx, consts.GitRevParse, gitVerifyFlag, ref)
	iox.Discard(out)

	return err == nil
}

// BranchExists reports whether a local branch ref exists.
func (client *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return false, fmt.Errorf(fmtValidateGitRefErr, err)
	}

	if branchRefVerified(ctx, client, branch) {
		return true, nil
	}

	if branchRefVerified(ctx, client, "refs/heads/"+branch) {
		return true, nil
	}

	return false, nil
}

// CheckoutBranch checks out an existing branch.
func (client *Client) CheckoutBranch(ctx context.Context, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = client.run(ctx, gitCheckout, branch)
	if err != nil {
		return fmt.Errorf(fmtCheckoutBranchErr, err)
	}

	return nil
}

func isNothingToCommit(err error) bool {
	return err != nil && strings.Contains(err.Error(), "nothing to commit")
}

// Commit creates a commit with the given message.
func (client *Client) Commit(ctx context.Context, message string) error {
	err := client.run(ctx, "commit", "-m", message)

	if isNothingToCommit(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// CreateOrResetBranch creates or resets a branch and checks it out.
func (client *Client) CreateOrResetBranch(ctx context.Context, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = client.run(ctx, gitCheckout, "-B", branch)
	if err != nil {
		return fmt.Errorf(fmtCheckoutBranchErr, err)
	}

	return nil
}

// DefaultBranch resolves the repository default branch from origin metadata.
func (client *Client) DefaultBranch(ctx context.Context) (string, error) {
	branch, err := client.defaultBranchFromOriginHEAD(ctx)
	if err == nil {
		return branch, nil
	}

	refreshErr := client.run(ctx, gitRemote, "set-head", consts.GitOrigin, "-a")

	branch, err = client.detectDefaultBranch(ctx)
	if err == nil {
		return branch, nil
	}

	return consts.Empty, fmt.Errorf("detect default branch: %w", defaultBranchFailure(refreshErr))
}

// EnsureSafeDirectory configures git safe.directory for the workspace when needed.
func (*Client) EnsureSafeDirectory() {
	// Safe directory is applied per command via -c; global/local config files are
	// not writable in GitHub Actions Docker containers (non-root user, read-only HOME).
}

// HasUnrelatedChanges reports whether the working tree has changes outside allowed paths.
func (client *Client) HasUnrelatedChanges(ctx context.Context, set pathSet) (bool, error) {
	out, err := client.output(ctx, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}

	return hasUnrelatedStatusLines(out, set), nil
}

// LastCommitMessage returns the subject of the latest commit on a branch.
func (client *Client) LastCommitMessage(ctx context.Context, branch string) (string, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return consts.Empty, fmt.Errorf(fmtValidateGitRefErr, err)
	}

	out, err := client.output(ctx, "log", "-1", "--format=%s", branch)
	if err != nil {
		return consts.Empty, fmt.Errorf("git log: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// Push pushes a branch to origin.
func (client *Client) Push(ctx context.Context, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = client.run(ctx, gitPush, consts.GitOrigin, branch)
	if err != nil {
		return fmt.Errorf(fmtGitPushErr, err)
	}

	return nil
}

// PushForceWithLease pushes a branch to origin with force-with-lease.
func (client *Client) PushForceWithLease(ctx context.Context, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = client.run(ctx, gitPush, "--force-with-lease", consts.GitOrigin, branch)
	if err != nil {
		return fmt.Errorf(fmtGitPushErr, err)
	}

	return nil
}

// Stage force-adds the given paths to the index.
func (client *Client) Stage(ctx context.Context, paths []string) error {
	if len(paths) == consts.IndexZero {
		return nil
	}

	err := validateStagePaths(client.workspace, paths)
	if err != nil {
		return fmt.Errorf("validate stage paths: %w", err)
	}

	err = client.runStageAdd(ctx, paths)
	if err != nil {
		return fmt.Errorf("run stage add: %w", err)
	}

	return nil
}

func (client *Client) branchFromCommand(ctx context.Context, args ...string) (string, bool) {
	out, err := client.output(ctx, args...)
	if err != nil {
		return consts.Empty, false
	}

	branch := normalizeBranch(out)

	return branch, isPlausibleDefaultBranch(branch)
}

func (client *Client) defaultBranchFromOriginHEAD(ctx context.Context) (string, error) {
	if branch, ok := client.originHEADFromSymbolicRef(ctx); ok {
		return branch, nil
	}

	if branch, ok := client.originHEADFromAbbrevRef(ctx); ok {
		return branch, nil
	}

	branch, err := client.originHEADFromCommit(ctx)
	if err != nil {
		return consts.Empty, fmt.Errorf("origin HEAD from commit: %w", err)
	}

	return branch, nil
}

func (client *Client) defaultBranchFromOriginHEADCommit(ctx context.Context) (string, error) {
	sha, err := client.originHEADSHA(ctx)
	if err != nil {
		return consts.Empty, fmt.Errorf("origin HEAD SHA: %w", err)
	}

	refs, err := client.refsAtOriginHEAD(ctx, sha)
	if err != nil {
		return consts.Empty, fmt.Errorf("refs at origin HEAD: %w", err)
	}

	branch, err := firstPlausibleRefOrError(refs)
	if err != nil {
		return consts.Empty, fmt.Errorf("first plausible ref: %w", err)
	}

	return branch, nil
}

func (client *Client) defaultBranchFromRemoteShow(ctx context.Context) (string, error) {
	out, err := client.output(ctx, gitRemote, "show", consts.GitOrigin)
	if err != nil {
		return consts.Empty, fmt.Errorf("git remote show: %w", err)
	}

	branch, ok := parseHEADBranchLine(out)

	if !ok {
		return consts.Empty, errHEADBranchNotFound
	}

	return branch, nil
}

func (client *Client) detectDefaultBranch(ctx context.Context) (string, error) {
	detectors := []func(context.Context) (string, error){
		client.defaultBranchFromOriginHEAD,
		client.defaultBranchFromRemoteShow,
	}

	for i := range detectors {
		branch, err := detectors[i](ctx)
		if err == nil {
			return branch, nil
		}
	}

	return consts.Empty, errDefaultBranchDetectionFailed
}

func (client *Client) gitArgs(args ...string) []string {
	return append([]string{
		gitConfigFlag, "safe.directory=" + client.workspace,
		gitConfigFlag, "user.email=" + commitUserEmail,
		gitConfigFlag, "user.name=" + commitUserName,
	}, args...)
}

func (client *Client) newGitCommand(ctx context.Context, args ...string) *exec.Cmd {
	gitArgs := client.gitArgs(args...)
	cmd := exec.CommandContext(ctx, gitBinary)

	cmd.Args = append([]string{gitBinary}, gitArgs...)
	cmd.Dir = client.workspace

	return cmd
}

func (client *Client) originHEADFromAbbrevRef(ctx context.Context) (string, bool) {
	return client.branchFromCommand(
		ctx,
		consts.GitRevParse,
		"--abbrev-ref",
		consts.GitRemoteHeadRef,
	)
}

func (client *Client) originHEADFromCommit(ctx context.Context) (string, error) {
	branch, err := client.defaultBranchFromOriginHEADCommit(ctx)
	if err == nil {
		return branch, nil
	}

	return consts.Empty, errOriginHEADNotAvailable
}

func (client *Client) originHEADFromSymbolicRef(ctx context.Context) (string, bool) {
	return client.branchFromCommand(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
}

func (client *Client) originHEADSHA(ctx context.Context) (string, error) {
	sha, err := client.output(ctx, consts.GitRevParse, consts.GitRemoteHeadRef)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve origin head sha: %w", err)
	}

	return strings.TrimSpace(sha), nil
}

func (client *Client) output(ctx context.Context, args ...string) (string, error) {
	cmd := client.newGitCommand(ctx, args...)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout

	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return consts.Empty, fmt.Errorf(
			fmtGitCmdErr,
			strings.Join(args, argSep),
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	return stdout.String(), nil
}

func (client *Client) refsAtOriginHEAD(ctx context.Context, sha string) (string, error) {
	refs, err := client.output(
		ctx,
		"for-each-ref",
		"--format=%(refname:short)",
		"refs/remotes/origin/",
		"--points-at",
		sha,
	)
	if err != nil {
		return consts.Empty, fmt.Errorf("list refs pointing at origin head: %w", err)
	}

	return refs, nil
}

func (client *Client) run(ctx context.Context, args ...string) error {
	cmd := client.newGitCommand(ctx, args...)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf(
			fmtGitCmdErr,
			strings.Join(args, argSep),
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	return nil
}

func (client *Client) runStageAdd(ctx context.Context, paths []string) error {
	args := append([]string{consts.GitAdd, "-f", "--"}, paths...)

	err := client.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("stage paths: %w", err)
	}

	return nil
}
