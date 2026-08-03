// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package github provides helpers for interacting with the GitHub API and Actions outputs.
package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v69/github"
	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/repo"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
	"golang.org/x/oauth2"
)

func sbWrite(w *strings.Builder, s string) {
	_, _ = w.WriteString(s) //nolint:errcheck // strings.Builder.WriteString cannot fail
}

func sbPrintf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) //nolint:errcheck // strings.Builder cannot fail
}

type (
	// PullRequest is a minimal view of a GitHub pull request.
	PullRequest struct {
		URL    string
		Number int
	}

	// Client wraps the GitHub API for TaskOtter sync operations.
	Client struct {
		api   *github.Client
		owner string
		repo  string
	}

	// StoreRef carries store reference metadata for PR rendering.
	StoreRef struct {
		SourceRef      string
		ResolvedCommit string
		DefaultBranch  string
	}
)

const (
	prTitle        = "chore(taskotter): sync taskfiles"
	outputFilePerm = 0o600
	fmtBulletItem  = "  - `%s`\n"
)

// ErrPullRequestNotFound indicates no open pull request exists for the branch.
var ErrPullRequestNotFound = errors.New("open pull request not found")

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
			Page:    consts.IndexZero,
			PerPage: consts.IndexZero,
		},
	}

	prs, _, err := c.api.PullRequests.List(ctx, c.owner, c.repo, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}

	if len(prs) == consts.IndexZero {
		return nil, ErrPullRequestNotFound
	}

	pr := prs[consts.IndexZero]

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
func StoreRefFrom(ref *store.RefInfo) *StoreRef {
	return &StoreRef{
		SourceRef:      ref.SourceRef,
		ResolvedCommit: ref.ResolvedCommit,
		DefaultBranch:  ref.DefaultBranch,
	}
}

// BuildPRBody renders the markdown body for a sync pull request.
func BuildPRBody(cfg *config.Config, plan *syncer.Plan, ref *StoreRef) string {
	var body strings.Builder

	sbWrite(&body, "## TaskOtter\n\n")

	writeMetadataSection(&body, cfg, ref)
	writeRequestedModulesSection(&body, cfg, plan)
	writeDependenciesSection(&body, plan.Dependencies)
	writeFileChangesSection(&body, plan)

	return body.String()
}

func writeMetadataSection(body *strings.Builder, cfg *config.Config, ref *StoreRef) {
	writeCoreMetadata(body, cfg, ref)
	writeJSRuntimeMetadata(body, cfg)

	sbWrite(body, consts.Newline)
}

func writeCoreMetadata(body *strings.Builder, cfg *config.Config, ref *StoreRef) {
	sbPrintf(body, "- Source: `%s`\n", config.StoreRepository)
	sbPrintf(body, "- Requested version: `%s`\n", emptyDash(cfg.StoreVersion))
	sbPrintf(body, "- Source reference: `%s`\n", ref.SourceRef)
	sbPrintf(body, "- Resolved commit: `%s`\n", ref.ResolvedCommit)
	sbPrintf(body, "- Default branch: `%s`\n", ref.DefaultBranch)
	sbPrintf(body, "- Target folder: `%s`\n", cfg.TargetFolder)
	sbPrintf(body, "- Documentation included: `%t`\n", cfg.IncludesDoc)
	sbPrintf(body, "- Root Taskfile synchronized: `%t`\n", cfg.SyncRoot)
	sbPrintf(body, "- JS runtime: `%s`\n", emptyDash(string(cfg.JSRuntime)))
}

func writeJSRuntimeMetadata(body *strings.Builder, cfg *config.Config) {
	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		return
	}

	sbPrintf(body, "- Package manager: `%s`\n", cfg.NodePackageManager)
	sbPrintf(body, "- Version manager: `%s`\n", cfg.NodeVersionManager)
}

func writeRequestedModulesSection(body *strings.Builder, cfg *config.Config, plan *syncer.Plan) {
	sbWrite(body, "### Requested modules\n\n")
	sbWrite(body, "| Task | Source module | Destination |\n")
	sbWrite(body, "|---|---|---|\n")

	tasks := cfg.Tasks

	for i := range tasks {
		task := tasks[i]
		rec := plan.Requested[task]
		sbPrintf(body, "| %s | `%s` | `%s` |\n", task, rec.SourceModule, rec.Path)
	}
}

func writeDependenciesSection(body *strings.Builder, deps []syncer.ModuleRecord) {
	sbWrite(body, "\n### Dependencies\n\n")
	sbWrite(body, "| Source module | Destination |\n")
	sbWrite(body, "|---|---|\n")

	for i := range deps {
		dep := &deps[i]
		sbPrintf(body, "| `%s` | `%s` |\n", dep.SourceModule, dep.Path)
	}
}

func writeFileChangesSection(body *strings.Builder, plan *syncer.Plan) {
	sbWrite(body, "\n### File changes\n\n")
	writeBulletGroup(body, "Added", plan.Added)
	writeBulletGroup(body, "Updated", plan.Updated)
	writeBulletGroup(body, "Removed", plan.Removed)
}

func writeBulletGroup(body *strings.Builder, label string, paths []string) {
	sbPrintf(body, "- %s: %d\n", label, len(paths))

	for i := range paths {
		sbPrintf(body, fmtBulletItem, paths[i])
	}
}

func emptyDash(v string) string {
	if v == consts.Empty {
		return consts.Empty
	}

	return v
}

// WriteOutputs writes GitHub Actions step outputs to path.
func WriteOutputs(path string, values map[string]string) error {
	if path == consts.Empty {
		return nil
	}

	var output strings.Builder

	for key := range values {
		sbWrite(&output, formatOutputLine(key, values[key]))
	}

	err := os.WriteFile(path, []byte(output.String()), outputFilePerm)
	if err != nil {
		return fmt.Errorf("write GitHub Actions outputs: %w", err)
	}

	return nil
}

func formatOutputLine(key, value string) string {
	if strings.Contains(value, consts.Newline) {
		return key + "<<EOF\n" + value + "\nEOF\n"
	}

	return key + "=" + value + consts.Newline
}
