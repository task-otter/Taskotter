// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package git provides helpers for running git commands used by the sync workflow.
package git

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

	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/pathutil"
)

type (
	// Operations abstracts git commands against a workspace checkout.
	Operations interface {
		EnsureSafeDirectory(ctx context.Context) error
		HasUnrelatedChanges(ctx context.Context, allowed map[string]struct{}) (bool, error)
		CheckoutBranch(ctx context.Context, branch string, create bool) error
		BranchExists(ctx context.Context, branch string) (bool, error)
		LastCommitMessage(ctx context.Context, branch string) (string, error)
		Stage(ctx context.Context, paths []string) error
		Commit(ctx context.Context, message string) error
		Push(ctx context.Context, branch string, forceWithLease bool) error
		DefaultBranch(ctx context.Context) (string, error)
	}

	// Client runs git commands in a workspace directory.
	Client struct {
		workspace string
	}
)

const (
	// SyncCommitMessage is the commit message TaskOtter uses for sync branches.
	SyncCommitMessage = "chore(taskotter): sync taskfiles"

	commitUserName  = "TaskOtter"
	commitUserEmail = "taskotter@users.noreply.github.com"

	gitStatusPathOffset = 3

	gitRemote                 = "remote"
	gitVerifyFlag             = "--verify"
	gitConfigFlag             = "-c"
	gitBinary                 = "git"
	fmtGitCmdErr              = "git %s: %w: %s"
	argSep                    = " "
	errMustNotStartWithHyphen = "%w: must not start with '-'"

	maxGitRefLen = 255
)

var (
	errOriginHEADNotAvailable       = errors.New("origin HEAD not available")
	errNoRemoteBranchAtOriginHEAD   = errors.New("no remote branch at origin HEAD commit")
	errHEADBranchNotFound           = errors.New("HEAD branch not found in remote show output")
	errDefaultBranchDetectionFailed = errors.New(
		"detect default branch: none of the detection methods succeeded",
	)
	errBranchNotOwned   = errors.New("branch exists but is not owned by TaskOtter")
	errInvalidGitRef    = errors.New("invalid git ref")
	errInvalidStagePath = errors.New("invalid stage path")

	gitRefPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
)

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

	_, err := pathutil.ValidateRelativePath(workspace, path)
	if err != nil {
		return fmt.Errorf("%w: %s", errInvalidStagePath, err)
	}

	return nil
}

// NewClient returns a git client bound to the given workspace path.
func NewClient(workspace string) *Client {
	return &Client{workspace: workspace}
}

// EnsureSafeDirectory configures git safe.directory for the workspace when needed.
func (c *Client) EnsureSafeDirectory(ctx context.Context) error {
	_ = ctx
	// Safe directory is applied per command via -c; global/local config files are
	// not writable in GitHub Actions Docker containers (non-root user, read-only HOME).
	return nil
}

// DefaultBranch resolves the repository default branch from origin metadata.
func (c *Client) DefaultBranch(ctx context.Context) (string, error) {
	branch, err := c.defaultBranchFromOriginHEAD(ctx)
	if err == nil {
		return branch, nil
	}

	refreshErr := c.run(ctx, gitRemote, "set-head", consts.GitOrigin, "-a")

	detectors := []func(context.Context) (string, error){
		c.defaultBranchFromOriginHEAD,
		c.defaultBranchFromRemoteShow,
	}

	for i := range detectors {
		branch, err = detectors[i](ctx)
		if err == nil {
			return branch, nil
		}
	}

	if refreshErr != nil {
		return consts.Empty, errors.Join(
			errDefaultBranchDetectionFailed,
			fmt.Errorf("refresh origin head: %w", refreshErr),
		)
	}

	return consts.Empty, errDefaultBranchDetectionFailed
}

// HasUnrelatedChanges reports whether the working tree has changes outside allowed paths.
func (c *Client) HasUnrelatedChanges(
	ctx context.Context,
	allowed map[string]struct{},
) (bool, error) {
	out, err := c.output(ctx, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}

	lines := strings.SplitSeq(strings.TrimSpace(out), consts.Newline)

	for line := range lines {
		path, ok := parseStatusPath(line)

		if !ok || isChangeAllowed(path, allowed) {
			continue
		}

		return true, nil
	}

	return false, nil
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

func isChangeAllowed(path string, allowed map[string]struct{}) bool {
	if _, ok := allowed[path]; ok {
		return true
	}

	return isAllowedPath(path, allowed)
}

// CheckoutBranch checks out a branch, optionally creating or resetting it.
func (c *Client) CheckoutBranch(ctx context.Context, branch string, create bool) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf("validate git ref: %w", err)
	}

	args := []string{"checkout"}

	if create {
		args = append(args, "-B")
	}

	args = append(args, branch)

	err = c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("checkout branch: %w", err)
	}

	return nil
}

// BranchExists reports whether a local branch ref exists.
func (c *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return false, fmt.Errorf("validate git ref: %w", err)
	}

	_, err = c.output(ctx, consts.GitRevParse, gitVerifyFlag, branch)
	if err == nil {
		return true, nil
	}

	_, err = c.output(ctx, consts.GitRevParse, gitVerifyFlag, "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}

	return false, nil
}

// LastCommitMessage returns the subject of the latest commit on a branch.
func (c *Client) LastCommitMessage(ctx context.Context, branch string) (string, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return consts.Empty, fmt.Errorf("validate git ref: %w", err)
	}

	out, err := c.output(ctx, "log", "-1", "--format=%s", branch)
	if err != nil {
		return consts.Empty, fmt.Errorf("git log: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// Stage force-adds the given paths to the index.
func (c *Client) Stage(ctx context.Context, paths []string) error {
	if len(paths) == consts.IndexZero {
		return nil
	}

	for i := range paths {
		err := ValidateStagePath(c.workspace, paths[i])
		if err != nil {
			return fmt.Errorf("validate stage path: %w", err)
		}
	}

	args := append([]string{consts.GitAdd, "-f", "--"}, paths...)

	err := c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("stage paths: %w", err)
	}

	return nil
}

// Commit creates a commit with the given message.
func (c *Client) Commit(ctx context.Context, message string) error {
	err := c.run(ctx, "commit", "-m", message)
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil
		}

		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// Push pushes a branch to origin, optionally with force-with-lease.
func (c *Client) Push(ctx context.Context, branch string, forceWithLease bool) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return fmt.Errorf("validate git ref: %w", err)
	}

	args := []string{"push"}

	if forceWithLease {
		args = append(args, "--force-with-lease")
	}

	args = append(args, consts.GitOrigin, branch)

	err = c.run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func (c *Client) gitArgs(args ...string) []string {
	return append([]string{
		gitConfigFlag, "safe.directory=" + c.workspace,
		gitConfigFlag, "user.email=" + commitUserEmail,
		gitConfigFlag, "user.name=" + commitUserName,
	}, args...)
}

func (c *Client) defaultBranchFromOriginHEAD(ctx context.Context) (string, error) {
	branch, ok := c.branchFromCommand(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")

	if ok {
		return branch, nil
	}

	branch, ok = c.branchFromCommand(
		ctx,
		consts.GitRevParse,
		"--abbrev-ref",
		consts.GitRemoteHeadRef,
	)

	if ok {
		return branch, nil
	}

	branch, err := c.defaultBranchFromOriginHEADCommit(ctx)
	if err == nil {
		return branch, nil
	}

	return "", errOriginHEADNotAvailable
}

func (c *Client) branchFromCommand(ctx context.Context, args ...string) (string, bool) {
	out, err := c.output(ctx, args...)
	if err != nil {
		return consts.Empty, false
	}

	branch := normalizeBranch(out)

	return branch, isPlausibleDefaultBranch(branch)
}

func (c *Client) defaultBranchFromOriginHEADCommit(ctx context.Context) (string, error) {
	sha, err := c.output(ctx, consts.GitRevParse, consts.GitRemoteHeadRef)
	if err != nil {
		return consts.Empty, fmt.Errorf("resolve origin head sha: %w", err)
	}

	refs, err := c.output(
		ctx,
		"for-each-ref",
		"--format=%(refname:short)",
		"refs/remotes/origin/",
		"--points-at",
		strings.TrimSpace(sha),
	)
	if err != nil {
		return consts.Empty, fmt.Errorf("list refs pointing at origin head: %w", err)
	}

	branch, ok := firstPlausibleRef(refs)

	if !ok {
		return consts.Empty, errNoRemoteBranchAtOriginHEAD
	}

	return branch, nil
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

func plausibleBranchFromLine(line string) (string, bool) {
	line = strings.TrimSpace(line)

	if line == consts.Empty || line == "origin/HEAD" {
		return consts.Empty, false
	}

	branch := normalizeBranch(line)

	return branch, isPlausibleDefaultBranch(branch)
}

func (c *Client) defaultBranchFromRemoteShow(ctx context.Context) (string, error) {
	out, err := c.output(ctx, gitRemote, "show", consts.GitOrigin)
	if err != nil {
		return consts.Empty, fmt.Errorf("git remote show: %w", err)
	}

	branch, ok := parseHEADBranchLine(out)

	if !ok {
		return consts.Empty, errHEADBranchNotFound
	}

	return branch, nil
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

func (c *Client) newGitCommand(ctx context.Context, args ...string) *exec.Cmd {
	gitArgs := c.gitArgs(args...)
	cmd := exec.CommandContext(ctx, gitBinary)

	cmd.Args = append([]string{gitBinary}, gitArgs...)
	cmd.Dir = c.workspace

	return cmd
}

func (c *Client) run(ctx context.Context, args ...string) error {
	cmd := c.newGitCommand(ctx, args...)

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

func (c *Client) output(ctx context.Context, args ...string) (string, error) {
	cmd := c.newGitCommand(ctx, args...)

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

// AllowedPathSet converts staged path strings into a lookup set.
func AllowedPathSet(paths []string) map[string]struct{} {
	out := make(map[string]struct{}, len(paths))

	for i := range paths {
		out[filepath.ToSlash(paths[i])] = struct{}{}
	}

	return out
}

// WriteLocalIdentity configures commit author metadata for sync commits.
func WriteLocalIdentity(ctx context.Context, ops Operations) error {
	_ = ctx
	_ = ops
	// Commit identity is applied per command via -c; config files are not writable
	// in GitHub Actions Docker containers.
	return nil
}

// IsGitRepo reports whether workspace contains a .git directory.
func IsGitRepo(workspace string) bool {
	_, err := os.Stat(filepath.Join(workspace, ".git"))

	return err == nil
}

// EnsureBranchOwned allows new sync branches and rejects foreign branch reuse.
func EnsureBranchOwned(ctx context.Context, ops Operations, branch string) error {
	exists, err := ops.BranchExists(ctx, branch)
	if err != nil {
		return fmt.Errorf("check branch exists: %w", err)
	}

	if !exists {
		return nil
	}

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

func checkBranchOwnership(msg, branch string) error {
	if msg != SyncCommitMessage {
		return fmt.Errorf("%w: %q", errBranchNotOwned, branch)
	}

	return nil
}

func normalizeBranch(name string) string {
	branch := strings.TrimSpace(name)

	return strings.TrimPrefix(branch, "origin/")
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

func isLikelyCommitSHA(branch string) bool {
	return len(branch) >= consts.IndexSeven && isHexString(branch)
}

func isHexString(value string) bool {
	for i := range len(value) {
		if !isHexDigit(rune(value[i])) {
			return false
		}
	}

	return true
}

func isHexDigit(r rune) bool {
	return isDigit(r) || isLowerHexLetter(r) || isUpperHexLetter(r)
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isLowerHexLetter(r rune) bool {
	return r >= 'a' && r <= 'f'
}

func isUpperHexLetter(r rune) bool {
	return r >= 'A' && r <= 'F'
}

func isAllowedPath(path string, allowed map[string]struct{}) bool {
	for allowedPath := range allowed {
		if path == allowedPath || strings.HasPrefix(path, allowedPath+"/") {
			return true
		}
	}

	return false
}
