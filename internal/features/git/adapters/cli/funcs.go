// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitports "github.com/task-otter/Taskotter/internal/features/git/ports"
	gitrefs "github.com/task-otter/Taskotter/internal/features/git/refs"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/pathutil"
	"github.com/task-otter/Taskotter/internal/shared/repo"
)

// NewClient returns a git client bound to the given workspace path.
func NewClient(workspace string) *Client {
	return newClient(workspace, gitBinary)
}

func newClient(workspace, binary string) *Client {
	return &Client{fns: clientFns{
		workspace: workspace,
		binary:    binary,
		run: func(ctx context.Context, args ...string) error {
			return runGitCommand(ctx, workspace, binary, args...)
		},
		output: func(ctx context.Context, args ...string) (string, error) {
			return outputGitCommand(ctx, workspace, binary, args...)
		},
	}}
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
func EnsureBranchOwned(ctx context.Context, ops gitports.BranchChecker, branch string) error {
	exists, err := ops.BranchExists(ctx, branch)
	if err != nil {
		return fmt.Errorf(errCheckBranchExists, err)
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
	err := gitrefs.Validate(ref)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
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
	iox.Discard(struct{}{})
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

	if line == consts.Empty || line == originHEADShortRef {
		return consts.Empty, false
	}

	branch := normalizeBranch(line)

	return branch, isPlausibleDefaultBranch(branch)
}

func isSafeRepoSegment(segment string) bool {
	if strings.Contains(segment, "..") {
		return false
	}

	return repoSegmentPattern.MatchString(segment)
}

func validateRepositoryCoordinate(repository string) error {
	owner, name, err := repo.Parse(repository)
	if err != nil {
		return fmt.Errorf("parse repository: %w", err)
	}

	if !isSafeRepoSegment(owner) || !isSafeRepoSegment(name) {
		return fmt.Errorf("%w %q", errInvalidRepository, repository)
	}

	return nil
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

func verifyExistingBranchOwned(
	ctx context.Context,
	ops gitports.BranchChecker,
	branch string,
) error {
	msg, err := ops.LastCommitMessage(ctx, branch)
	if err != nil {
		return fmt.Errorf(errReadLastCommitMessage, err)
	}

	err = checkBranchOwnership(msg, branch)
	if err != nil {
		return fmt.Errorf("check branch ownership: %w", err)
	}

	return nil
}

func branchRefVerified(ctx context.Context, client *Client, ref string) bool {
	out, err := output(ctx, client, consts.GitRevParse, gitVerifyFlag, ref)
	iox.Discard(out)

	return err == nil
}

// BranchExists reports whether a local branch ref exists.
func (client *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	iox.Discard(client.fns)

	exists, err := branchExists(ctx, client, branch)
	if err != nil {
		return false, fmt.Errorf(errCheckBranchExists, err)
	}

	return exists, nil
}

func branchExists(ctx context.Context, client *Client, branch string) (bool, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return false, fmt.Errorf(fmtValidateGitRefErr, err)
	}

	if branchExistsAnyRef(ctx, client, branch) {
		return true, nil
	}

	return false, nil
}

func (client *Client) runOp(op string, err error) error {
	iox.Discard(client.fns)

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// CheckoutBranch checks out an existing branch.
func (client *Client) CheckoutBranch(ctx context.Context, branch string) error {
	return client.runOp("checkout branch", checkoutBranch(ctx, client, branch))
}

func checkoutBranch(ctx context.Context, client *Client, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = run(ctx, client, gitCheckout, branch)
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
	return client.runOp("commit changes", commit(ctx, client, message))
}

func commit(ctx context.Context, client *Client, message string) error {
	err := run(ctx, client, "commit", "-m", message)

	if isNothingToCommit(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// ConfigureCredentials clears checkout extraheader auth and sets origin to a token URL.
func (client *Client) ConfigureCredentials(ctx context.Context, token, repository string) error {
	iox.Discard(client.fns)

	err := configureCredentials(
		ctx,
		client,
		&credArgs{token: token, repository: repository},
	)
	if err != nil {
		return fmt.Errorf("configure credentials: %w", err)
	}

	return nil
}

func configureCredentials(ctx context.Context, client *Client, creds *credArgs) error {
	if creds.token == consts.Empty || creds.repository == consts.Empty {
		return nil
	}

	err := applyOriginCredentials(ctx, client, creds)
	if err != nil {
		return fmt.Errorf("apply origin credentials: %w", err)
	}

	return nil
}

func applyOriginCredentials(ctx context.Context, client *Client, creds *credArgs) error {
	err := validateRepositoryCoordinate(creds.repository)
	if err != nil {
		return fmt.Errorf("validate creds.repository: %w", err)
	}

	clearGitHubExtraHeader(ctx, client)

	err = setOriginRemoteURL(
		ctx,
		client,
		fmt.Sprintf(originAccessURLFmt, creds.token, creds.repository),
	)
	if err != nil {
		return fmt.Errorf("set origin url: %w", err)
	}

	return nil
}

// CreateOrResetBranch creates or resets a branch and checks it out.
func (client *Client) CreateOrResetBranch(ctx context.Context, branch string) error {
	return client.runOp("create or reset branch", createOrResetBranch(ctx, client, branch))
}

func createOrResetBranch(ctx context.Context, client *Client, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = run(ctx, client, gitCheckout, "-B", branch)
	if err != nil {
		return fmt.Errorf(fmtCheckoutBranchErr, err)
	}

	return nil
}

// DefaultBranch resolves the repository default branch from origin metadata.
func (client *Client) DefaultBranch(ctx context.Context) (string, error) {
	iox.Discard(client.fns)

	branch, err := defaultBranch(ctx, client)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve default branch: %w", err)
	}

	return branch, nil
}

func defaultBranch(ctx context.Context, client *Client) (string, error) {
	branch, err := defaultBranchFromOriginHEAD(ctx, client)
	if err == nil {
		return branch, nil
	}

	refreshErr := run(ctx, client, gitRemote, "set-head", consts.GitOrigin, "-a")

	branch, err = detectDefaultBranch(ctx, client)
	if err == nil {
		return branch, nil
	}

	return consts.Empty, fmt.Errorf("detect default branch: %w", defaultBranchFailure(refreshErr))
}

// EnsureSafeDirectory configures git safe.directory for the workspace when needed.
func (*Client) EnsureSafeDirectory() {
	iox.Discard(struct{}{})
}

// HasUnrelatedChanges reports whether the working tree has changes outside allowed paths.
func (client *Client) HasUnrelatedChanges(ctx context.Context, set pathSet) (bool, error) {
	iox.Discard(client.fns)

	changed, err := hasUnrelatedChanges(ctx, client, set)
	if err != nil {
		return false, fmt.Errorf("check unrelated changes: %w", err)
	}

	return changed, nil
}

func hasUnrelatedChanges(ctx context.Context, client *Client, set pathSet) (bool, error) {
	out, err := output(ctx, client, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}

	return hasUnrelatedStatusLines(out, set), nil
}

// LastCommitMessage returns the subject of the latest commit on a branch.
func (client *Client) LastCommitMessage(ctx context.Context, branch string) (string, error) {
	iox.Discard(client.fns)

	message, err := lastCommitMessage(ctx, client, branch)
	if err != nil {
		return consts.Empty, fmt.Errorf(errReadLastCommitMessage, err)
	}

	return message, nil
}

func lastCommitMessage(ctx context.Context, client *Client, branch string) (string, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return consts.Empty, fmt.Errorf(fmtValidateGitRefErr, err)
	}

	out, err := output(ctx, client, "log", "-1", "--format=%s", branch)
	if err != nil {
		return consts.Empty, fmt.Errorf("git log: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// Push pushes a branch to origin.
func (client *Client) Push(ctx context.Context, branch string) error {
	return client.runOp("push branch", push(ctx, client, branch))
}

func push(ctx context.Context, client *Client, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = run(ctx, client, gitPush, consts.GitOrigin, branch)
	if err != nil {
		return fmt.Errorf(fmtGitPushErr, err)
	}

	return nil
}

// PushForceWithLease pushes a branch to origin with force-with-lease.
func (client *Client) PushForceWithLease(ctx context.Context, branch string) error {
	return client.runOp(
		"push branch with force-with-lease",
		pushForceWithLease(ctx, client, branch),
	)
}

func pushForceWithLease(ctx context.Context, client *Client, branch string) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf(fmtValidateGitRefErr, err)
	}

	err = run(ctx, client, gitPush, "--force-with-lease", consts.GitOrigin, branch)
	if err != nil {
		return fmt.Errorf(fmtGitPushErr, err)
	}

	return nil
}

// Stage force-adds the given paths to the index.
func (client *Client) Stage(ctx context.Context, paths []string) error {
	iox.Discard(client.fns)

	err := stage(ctx, client, paths)
	if err != nil {
		return fmt.Errorf(errStagePaths, err)
	}

	return nil
}

func stage(ctx context.Context, client *Client, paths []string) error {
	if len(paths) == consts.IndexZero {
		return nil
	}

	err := validateStagePaths(client.fns.workspace, paths)
	if err != nil {
		return fmt.Errorf("validate stage paths: %w", err)
	}

	err = runStageAdd(ctx, client, paths)
	if err != nil {
		return fmt.Errorf("run stage add: %w", err)
	}

	return nil
}

func branchExistsAnyRef(ctx context.Context, client *Client, branch string) bool {
	return branchRefVerified(ctx, client, branch) ||
		branchRefVerified(ctx, client, "refs/heads/"+branch)
}

func branchFromCommand(ctx context.Context, client *Client, args ...string) (string, bool) {
	out, err := output(ctx, client, args...)
	if err != nil {
		return consts.Empty, false
	}

	branch := normalizeBranch(out)

	return branch, isPlausibleDefaultBranch(branch)
}

func clearGitHubExtraHeader(ctx context.Context, client *Client) {
	iox.Discard(
		run(ctx, client, consts.GitConfig, gitConfigLocal, gitConfigUnsetAll, httpExtraHeaderKey),
	)
}

func defaultBranchFromOriginHEAD(ctx context.Context, client *Client) (string, error) {
	if branch, ok := originHEADFromSymbolicRef(ctx, client); ok {
		return branch, nil
	}

	if branch, ok := originHEADFromAbbrevRef(ctx, client); ok {
		return branch, nil
	}

	branch, err := originHEADFromCommit(ctx, client)
	if err != nil {
		return consts.Empty, fmt.Errorf("origin HEAD from commit: %w", err)
	}

	return branch, nil
}

func defaultBranchFromOriginHEADCommit(ctx context.Context, client *Client) (string, error) {
	sha, err := originHEADSHA(ctx, client)
	if err != nil {
		return consts.Empty, fmt.Errorf("origin HEAD SHA: %w", err)
	}

	refs, err := refsAtOriginHEAD(ctx, client, sha)
	if err != nil {
		return consts.Empty, fmt.Errorf("refs at origin HEAD: %w", err)
	}

	branch, err := firstPlausibleRefOrError(refs)
	if err != nil {
		return consts.Empty, fmt.Errorf("first plausible ref: %w", err)
	}

	return branch, nil
}

func defaultBranchFromRemoteShow(ctx context.Context, client *Client) (string, error) {
	out, err := output(ctx, client, gitRemote, "show", consts.GitOrigin)
	if err != nil {
		return consts.Empty, fmt.Errorf("git remote show: %w", err)
	}

	branch, ok := parseHEADBranchLine(out)

	if !ok {
		return consts.Empty, errHEADBranchNotFound
	}

	return branch, nil
}

func detectDefaultBranch(ctx context.Context, client *Client) (string, error) {
	branch, err := defaultBranchFromOriginHEAD(ctx, client)
	if err == nil {
		return branch, nil
	}

	branch, err = defaultBranchFromRemoteShow(ctx, client)
	if err == nil {
		return branch, nil
	}

	return consts.Empty, errDefaultBranchDetectionFailed
}

func gitArgs(workspace string, args ...string) []string {
	return append([]string{
		gitConfigFlag, "safe.directory=" + workspace,
		gitConfigFlag, "user.email=" + commitUserEmail,
		gitConfigFlag, "user.name=" + commitUserName,
	}, args...)
}

func newGitCommand(ctx context.Context, workspace, binary string, args ...string) *exec.Cmd {
	cmdArgs := gitArgs(workspace, args...)
	cmd := exec.CommandContext(ctx, binary)

	cmd.Args = append([]string{binary}, cmdArgs...)
	cmd.Dir = workspace

	return cmd
}

func originHEADFromAbbrevRef(ctx context.Context, client *Client) (string, bool) {
	return branchFromCommand(ctx, client,
		consts.GitRevParse,
		"--abbrev-ref",
		consts.GitRemoteHeadRef,
	)
}

func originHEADFromCommit(ctx context.Context, client *Client) (string, error) {
	branch, err := defaultBranchFromOriginHEADCommit(ctx, client)
	if err == nil {
		return branch, nil
	}

	return consts.Empty, errOriginHEADNotAvailable
}

func originHEADFromSymbolicRef(ctx context.Context, client *Client) (string, bool) {
	return branchFromCommand(ctx, client, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
}

func originHEADSHA(ctx context.Context, client *Client) (string, error) {
	sha, err := output(ctx, client, consts.GitRevParse, consts.GitRemoteHeadRef)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve origin head sha: %w", err)
	}

	return strings.TrimSpace(sha), nil
}

func (client *Client) output(ctx context.Context, args ...string) (string, error) {
	iox.Discard(client.fns)

	out, err := client.fns.output(ctx, args...)
	if err != nil {
		return consts.Empty, fmt.Errorf(errRunGitOutput, err)
	}

	return out, nil
}

func outputGitCommand(
	ctx context.Context,
	workspace, binary string,
	args ...string,
) (string, error) {
	cmd := newGitCommand(ctx, workspace, binary, args...)

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

func refsAtOriginHEAD(ctx context.Context, client *Client, sha string) (string, error) {
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

func runGitCommand(ctx context.Context, workspace, binary string, args ...string) error {
	cmd := newGitCommand(ctx, workspace, binary, args...)

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

func runStageAdd(ctx context.Context, client *Client, paths []string) error {
	args := append([]string{consts.GitAdd, "-f", "--"}, paths...)

	err := run(ctx, client, args...)
	if err != nil {
		return fmt.Errorf(errStagePaths, err)
	}

	return nil
}

func setOriginRemoteURL(ctx context.Context, client *Client, remoteURL string) error {
	cmd := newGitCommand(
		ctx,
		client.fns.workspace,
		client.fns.binary,
		gitRemote,
		"set-url",
		consts.GitOrigin,
		remoteURL,
	)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("git remote set-url origin: %w", err)
	}

	return nil
}

func run(ctx context.Context, client *Client, args ...string) error {
	err := client.fns.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("run git command: %w", err)
	}

	return nil
}

func output(ctx context.Context, client *Client, args ...string) (string, error) {
	out, err := client.fns.output(ctx, args...)
	if err != nil {
		return consts.Empty, fmt.Errorf(errRunGitOutput, err)
	}

	return out, nil
}
