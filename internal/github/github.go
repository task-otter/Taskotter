// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package github

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/repo"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
	"golang.org/x/oauth2"
)

const (
	prTitle        = "chore(taskotter): sync taskfiles"
	outputFilePerm = 0o600
)

// ErrPullRequestNotFound indicates no open pull request exists for the branch.
var ErrPullRequestNotFound = errors.New("open pull request not found")

// PullRequest is a minimal view of a GitHub pull request.
type PullRequest struct {
	URL    string
	Number int
}

// Client wraps the GitHub API for TaskOtter sync operations.
type Client struct {
	api   *github.Client
	owner string
	repo  string
}

// StoreRef carries store reference metadata for PR rendering.
type StoreRef struct {
	SourceRef      string
	ResolvedCommit string
	DefaultBranch  string
}

// NewClient creates a GitHub API client for repository.
func NewClient(ctx context.Context, token, repository string) (*Client, error) {
	owner, repoName, err := repo.Parse(repository)
	if err != nil {
		return nil, fmt.Errorf("parse repository: %w", err)
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken:  token,
		TokenType:    "Bearer",
		RefreshToken: "",
		Expiry:       time.Time{},
		ExpiresIn:    0,
	})

	return &Client{
		api:   github.NewClient(oauth2.NewClient(ctx, tokenSource)),
		owner: owner,
		repo:  repoName,
	}, nil
}

// FindOpenPR returns an open pull request for branch into base, if one exists.
func (c *Client) FindOpenPR(ctx context.Context, branch, base string) (*PullRequest, error) {
	head := fmt.Sprintf("%s:%s", c.owner, branch)

	listOpts := &github.PullRequestListOptions{
		State:     "open",
		Head:      head,
		Base:      base,
		Sort:      "",
		Direction: "",
		ListOptions: github.ListOptions{
			Page:    0,
			PerPage: 0,
		},
	}

	prs, _, err := c.api.PullRequests.List(ctx, c.owner, c.repo, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}

	if len(prs) == 0 {
		return nil, ErrPullRequestNotFound
	}

	pr := prs[0]

	return &PullRequest{Number: pr.GetNumber(), URL: pr.GetHTMLURL()}, nil
}

// CreatePR opens a new pull request from branch into base.
func (c *Client) CreatePR(ctx context.Context, branch, base, body string) (*PullRequest, error) {
	newPR := &github.NewPullRequest{
		Title:               new(prTitle),
		Head:                new(branch),
		Base:                new(base),
		Body:                new(body),
		HeadRepo:            nil,
		Issue:               nil,
		MaintainerCanModify: nil,
		Draft:               nil,
	}

	pr, _, err := c.api.PullRequests.Create(ctx, c.owner, c.repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	return &PullRequest{Number: pr.GetNumber(), URL: pr.GetHTMLURL()}, nil
}

// UpdatePRBody replaces the body of an existing pull request.
func (c *Client) UpdatePRBody(ctx context.Context, number int, body string) error {
	var edit github.PullRequest

	edit.Body = new(body)

	_, _, err := c.api.PullRequests.Edit(ctx, c.owner, c.repo, number, &edit)
	if err != nil {
		return fmt.Errorf("update pull request: %w", err)
	}

	return nil
}

// StoreRefFrom converts store reference metadata for PR rendering.
func StoreRefFrom(ref store.RefInfo) StoreRef {
	return StoreRef{
		SourceRef:      ref.SourceRef,
		ResolvedCommit: ref.ResolvedCommit,
		DefaultBranch:  ref.DefaultBranch,
	}
}

// BuildPRBody renders the markdown body for a sync pull request.
func BuildPRBody(cfg *config.Config, plan *syncer.Plan, ref StoreRef) string {
	var body strings.Builder

	body.WriteString("## TaskOtter\n\n")
	fmt.Fprintf(&body, "- Source: `%s`\n", config.StoreRepository)
	fmt.Fprintf(&body, "- Requested version: `%s`\n", emptyDash(cfg.StoreVersion))
	fmt.Fprintf(&body, "- Source reference: `%s`\n", ref.SourceRef)
	fmt.Fprintf(&body, "- Resolved commit: `%s`\n", ref.ResolvedCommit)
	fmt.Fprintf(&body, "- Default branch: `%s`\n", ref.DefaultBranch)
	fmt.Fprintf(&body, "- Target folder: `%s`\n", cfg.TargetFolder)
	fmt.Fprintf(&body, "- Documentation included: `%t`\n", cfg.IncludesDoc)
	fmt.Fprintf(&body, "- Root Taskfile synchronized: `%t`\n", cfg.SyncRoot)
	fmt.Fprintf(&body, "- JS runtime: `%s`\n", emptyDash(string(cfg.JSRuntime)))

	if cfg.JSRuntime == config.JSRuntimeNodeJS {
		fmt.Fprintf(&body, "- Package manager: `%s`\n", cfg.NodePackageManager)
		fmt.Fprintf(&body, "- Version manager: `%s`\n", cfg.NodeVersionManager)
	}

	body.WriteString("\n")

	body.WriteString("### Requested modules\n\n")
	body.WriteString("| Task | Source module | Destination |\n")
	body.WriteString("|---|---|---|\n")

	for _, task := range cfg.Tasks {
		rec := plan.Requested[task]
		fmt.Fprintf(&body, "| %s | `%s` | `%s` |\n", task, rec.SourceModule, rec.Path)
	}

	body.WriteString("\n### Dependencies\n\n")
	body.WriteString("| Source module | Destination |\n")
	body.WriteString("|---|---|\n")

	for _, dep := range plan.Dependencies {
		fmt.Fprintf(&body, "| `%s` | `%s` |\n", dep.SourceModule, dep.Path)
	}

	body.WriteString("\n### File changes\n\n")
	fmt.Fprintf(&body, "- Added: %d\n", len(plan.Added))

	for _, path := range plan.Added {
		fmt.Fprintf(&body, "  - `%s`\n", path)
	}

	fmt.Fprintf(&body, "- Updated: %d\n", len(plan.Updated))

	for _, path := range plan.Updated {
		fmt.Fprintf(&body, "  - `%s`\n", path)
	}

	fmt.Fprintf(&body, "- Removed: %d\n", len(plan.Removed))

	for _, path := range plan.Removed {
		fmt.Fprintf(&body, "  - `%s`\n", path)
	}

	return body.String()
}

func emptyDash(v string) string {
	if v == "" {
		return ""
	}

	return v
}

// WriteOutputs writes GitHub Actions step outputs to path.
func WriteOutputs(path string, values map[string]string) error {
	if path == "" {
		return nil
	}

	var output strings.Builder

	for key, value := range values {
		if strings.Contains(value, "\n") {
			output.WriteString(key)
			output.WriteString("<<EOF\n")
			output.WriteString(value)
			output.WriteString("\nEOF\n")

			continue
		}

		output.WriteString(key)
		output.WriteString("=")
		output.WriteString(value)
		output.WriteString("\n")
	}

	err := os.WriteFile(path, []byte(output.String()), outputFilePerm)
	if err != nil {
		return fmt.Errorf("write GitHub Actions outputs: %w", err)
	}

	return nil
}
