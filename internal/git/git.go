// Package git wraps workspace git operations used during TaskOtter sync.
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

	"github.com/task-otter/Taskotter/internal/pathutil"
)

const (
	// SyncCommitMessage is the commit message TaskOtter uses for sync branches.
	SyncCommitMessage = "chore(taskotter): sync taskfiles"

	commitUserName  = "TaskOtter"
	commitUserEmail = "taskotter@users.noreply.github.com"

	gitStatusPathOffset = 3
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
)

const maxGitRefLen = 255

var gitRefPattern = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)

// ValidateGitRef checks that ref is safe to pass to git commands.
func ValidateGitRef(ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("%w: must not be empty", errInvalidGitRef)
	}

	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("%w: must not start with '-'", errInvalidGitRef)
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
	if strings.HasPrefix(trimmed, "-") {
		return fmt.Errorf("%w: must not start with '-'", errInvalidStagePath)
	}

	_, err := pathutil.ValidateRelativePath(workspace, path)
	if err != nil {
		return fmt.Errorf("%w: %w", errInvalidStagePath, err)
	}

	return nil
}

// Operations abstracts git commands against a workspace checkout.
type Operations interface {
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
type Client struct {
	workspace string
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

	_ = c.run(ctx, "remote", "set-head", "origin", "-a")

	branch, err = c.defaultBranchFromOriginHEAD(ctx)
	if err == nil {
		return branch, nil
	}

	branch, err = c.defaultBranchFromRemoteShow(ctx)
	if err == nil {
		return branch, nil
	}

	return "", errDefaultBranchDetectionFailed
}

// HasUnrelatedChanges reports whether the working tree has changes outside allowed paths.
func (c *Client) HasUnrelatedChanges(
	ctx context.Context,
	allowed map[string]struct{},
) (bool, error) {
	out, err := c.output(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}

	lines := strings.SplitSeq(strings.TrimSpace(out), "\n")
	for line := range lines {
		if line == "" {
			continue
		}

		if len(line) < gitStatusPathOffset {
			continue
		}

		path := strings.TrimSpace(line[gitStatusPathOffset:])
		if path == "" {
			continue
		}

		if _, ok := allowed[path]; ok {
			continue
		}

		if isAllowedPath(path, allowed) {
			continue
		}

		return true, nil
	}

	return false, nil
}

// CheckoutBranch checks out a branch, optionally creating or resetting it.
func (c *Client) CheckoutBranch(ctx context.Context, branch string, create bool) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return err
	}

	args := []string{"checkout"}
	if create {
		args = append(args, "-B")
	}

	args = append(args, branch)

	return c.run(ctx, args...)
}

// BranchExists reports whether a local branch ref exists.
func (c *Client) BranchExists(ctx context.Context, branch string) (bool, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return false, err
	}

	_, err = c.output(ctx, "rev-parse", "--verify", branch)
	if err == nil {
		return true, nil
	}

	_, err = c.output(ctx, "rev-parse", "--verify", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}

	return false, nil
}

// LastCommitMessage returns the subject of the latest commit on a branch.
func (c *Client) LastCommitMessage(ctx context.Context, branch string) (string, error) {
	err := ValidateGitRef(branch)
	if err != nil {
		return "", err
	}

	out, err := c.output(ctx, "log", "-1", "--format=%s", branch)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

// Stage force-adds the given paths to the index.
func (c *Client) Stage(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	for _, path := range paths {
		err := ValidateStagePath(c.workspace, path)
		if err != nil {
			return err
		}
	}

	args := append([]string{"add", "-f", "--"}, paths...)

	return c.run(ctx, args...)
}

// Commit creates a commit with the given message.
func (c *Client) Commit(ctx context.Context, message string) error {
	err := c.run(ctx, "commit", "-m", message)
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil
		}

		return err
	}

	return nil
}

// Push pushes a branch to origin, optionally with force-with-lease.
func (c *Client) Push(ctx context.Context, branch string, forceWithLease bool) error {
	err := ValidateGitRef(branch)
	if err != nil {
		return err
	}

	args := []string{"push"}
	if forceWithLease {
		args = append(args, "--force-with-lease")
	}

	args = append(args, "origin", branch)

	return c.run(ctx, args...)
}

func (c *Client) gitArgs(args ...string) []string {
	return append([]string{
		"-c", "safe.directory=" + c.workspace,
		"-c", "user.email=" + commitUserEmail,
		"-c", "user.name=" + commitUserName,
	}, args...)
}

func (c *Client) defaultBranchFromOriginHEAD(ctx context.Context) (string, error) {
	out, err := c.output(ctx, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		if branch := normalizeBranch(out); isPlausibleDefaultBranch(branch) {
			return branch, nil
		}
	}

	out, err = c.output(ctx, "rev-parse", "--abbrev-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		if branch := normalizeBranch(out); isPlausibleDefaultBranch(branch) {
			return branch, nil
		}
	}

	branch, err := c.defaultBranchFromOriginHEADCommit(ctx)
	if err == nil {
		return branch, nil
	}

	return "", errOriginHEADNotAvailable
}

func (c *Client) defaultBranchFromOriginHEADCommit(ctx context.Context) (string, error) {
	sha, err := c.output(ctx, "rev-parse", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", err
	}

	sha = strings.TrimSpace(sha)

	refs, err := c.output(
		ctx,
		"for-each-ref",
		"--format=%(refname:short)",
		"refs/remotes/origin/",
		"--points-at",
		sha,
	)
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(strings.TrimSpace(refs), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "origin/HEAD" {
			continue
		}

		if branch := normalizeBranch(line); isPlausibleDefaultBranch(branch) {
			return branch, nil
		}
	}

	return "", errNoRemoteBranchAtOriginHEAD
}

func (c *Client) defaultBranchFromRemoteShow(ctx context.Context) (string, error) {
	out, err := c.output(ctx, "remote", "show", "origin")
	if err != nil {
		return "", err
	}

	const prefix = "HEAD branch: "

	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, prefix); ok {
			if branch := strings.TrimSpace(after); branch != "" {
				return branch, nil
			}
		}
	}

	return "", errHEADBranchNotFound
}

func (c *Client) newGitCommand(ctx context.Context, args ...string) *exec.Cmd {
	gitArgs := c.gitArgs(args...)
	cmd := exec.CommandContext(ctx, "git")

	cmd.Args = append([]string{"git"}, gitArgs...)
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
			"git %s: %w: %s",
			strings.Join(args, " "),
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
		return "", fmt.Errorf(
			"git %s: %w: %s",
			strings.Join(args, " "),
			err,
			strings.TrimSpace(stderr.String()),
		)
	}

	return stdout.String(), nil
}

// AllowedPathSet converts staged path strings into a lookup set.
func AllowedPathSet(paths []string) map[string]struct{} {
	out := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		out[filepath.ToSlash(path)] = struct{}{}
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
	if branch == "" || branch == "HEAD" || branch == "origin" {
		return false
	}

	if len(branch) >= 7 && isHexString(branch) {
		return false
	}

	return true
}

func isHexString(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}

	return true
}

func isAllowedPath(path string, allowed map[string]struct{}) bool {
	for allowedPath := range allowed {
		if path == allowedPath || strings.HasPrefix(path, allowedPath+"/") {
			return true
		}
	}

	return false
}
