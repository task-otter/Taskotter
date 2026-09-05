// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package github implements the PR feature ports against the GitHub REST API.
package github

import (
	"context"
	"fmt"

	prdomain "github.com/task-otter/Taskotter/internal/features/pr/domain"
	"github.com/task-otter/Taskotter/internal/features/pr/ports"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/githubapi"
	"github.com/task-otter/Taskotter/internal/shared/repo"
)

type (
	clientFns = struct {
		createPR   func(context.Context, *ports.CreatePRRequest) (*ports.PullRequest, error)
		findOpenPR func(context.Context, string, string) (*ports.PullRequest, error)
		updateBody func(context.Context, int, string) error
	}

	listOpenPRArgs = struct {
		api    *githubapi.Client
		owner  string
		repo   string
		branch string
		base   string
	}

	// Client wraps the GitHub API for TaskOtter sync operations.
	Client struct {
		fns clientFns
	}
)

const (
	prTitle          = "chore(taskotter): sync taskfiles"
	errFmtCreatePR   = "create pull request: %w"
	errFmtFindOpenPR = "find open pull request: %w"
)

// NewClient creates a GitHub API client for repository.
func NewClient(ctx context.Context, token, repository string) (*Client, error) {
	owner, repoName, err := repo.Parse(repository)
	if err != nil {
		return nil, fmt.Errorf("parse repository: %w", err)
	}

	api := githubapi.NewClient(ctx, token)

	return &Client{fns: clientFns{
		createPR:   makeCreatePR(api, owner, repoName),
		findOpenPR: makeFindOpenPR(api, owner, repoName),
		updateBody: makeUpdateBody(api, owner, repoName),
	}}, nil
}

// CreatePR opens a new pull request from branch into base.
func (client *Client) CreatePR(
	ctx context.Context,
	req *ports.CreatePRRequest,
) (*ports.PullRequest, error) {
	pull, err := client.fns.createPR(ctx, req)
	if err != nil {
		return nil, fmt.Errorf(errFmtCreatePR, err)
	}

	return pull, nil
}

// FindOpenPR returns an open pull request for branch into base, if one exists.
func (client *Client) FindOpenPR(
	ctx context.Context,
	branch, base string,
) (*ports.PullRequest, error) {
	pull, err := client.fns.findOpenPR(ctx, branch, base)
	if err != nil {
		return nil, fmt.Errorf(errFmtFindOpenPR, err)
	}

	return pull, nil
}

// UpdatePRBody replaces the body of an existing pull request.
func (client *Client) UpdatePRBody(ctx context.Context, number int, body string) error {
	err := client.fns.updateBody(ctx, number, body)
	if err != nil {
		return fmt.Errorf("update pull request body: %w", err)
	}

	return nil
}

func makeCreatePR(
	api *githubapi.Client,
	owner, repoName string,
) func(context.Context, *ports.CreatePRRequest) (*ports.PullRequest, error) {
	return func(ctx context.Context, req *ports.CreatePRRequest) (*ports.PullRequest, error) {
		pull, err := api.CreatePR(ctx, &githubapi.CreatePROptions{
			Owner: owner,
			Repo:  repoName,
			Title: prTitle,
			Head:  req.Branch,
			Base:  req.Base,
			Body:  req.Body,
		})
		if err != nil {
			return nil, fmt.Errorf(errFmtCreatePR, err)
		}

		return pullRequestFromAPI(pull), nil
	}
}

func makeFindOpenPR(
	api *githubapi.Client,
	owner, repoName string,
) func(context.Context, string, string) (*ports.PullRequest, error) {
	return func(ctx context.Context, branch, base string) (*ports.PullRequest, error) {
		pullRequests, err := listOpenPRsForBranch(ctx, &listOpenPRArgs{
			api: api, owner: owner, repo: repoName, branch: branch, base: base,
		})
		if err != nil {
			return nil, fmt.Errorf("list pull requests: %w", err)
		}

		pr, err := openPullRequestFromList(pullRequests)
		if err != nil {
			return nil, fmt.Errorf("select open pull request: %w", err)
		}

		return pr, nil
	}
}

func makeUpdateBody(
	api *githubapi.Client,
	owner, repoName string,
) func(context.Context, int, string) error {
	return func(ctx context.Context, number int, body string) error {
		err := api.EditPRBody(ctx, &githubapi.EditPRBodyOptions{
			Owner:  owner,
			Repo:   repoName,
			Number: number,
			Body:   body,
		})
		if err != nil {
			return fmt.Errorf("update pull request: %w", err)
		}

		return nil
	}
}

func listOpenPRsForBranch(
	ctx context.Context,
	args *listOpenPRArgs,
) ([]githubapi.PullRequest, error) {
	pullRequests, err := args.api.ListOpenPRs(ctx, &githubapi.ListOpenPROptions{
		Owner: args.owner,
		Repo:  args.repo,
		Head:  fmt.Sprintf("%s:%s", args.owner, args.branch),
		Base:  args.base,
	})
	if err != nil {
		return nil, fmt.Errorf("list open pull requests: %w", err)
	}

	return pullRequests, nil
}

func firstOpenPullRequest(pullRequests []githubapi.PullRequest) (*ports.PullRequest, error) {
	if len(pullRequests) == consts.IndexZero {
		return nil, prdomain.ErrPullRequestNotFound
	}

	return pullRequestFromAPI(pullRequests[consts.IndexZero]), nil
}

func openPullRequestFromList(pullRequests []githubapi.PullRequest) (*ports.PullRequest, error) {
	pr, err := firstOpenPullRequest(pullRequests)
	if err != nil {
		return nil, fmt.Errorf(errFmtFindOpenPR, err)
	}

	return pr, nil
}

func pullRequestFromAPI(pull githubapi.PullRequest) *ports.PullRequest {
	return &ports.PullRequest{Number: pull.Number, URL: pull.HTMLURL}
}
