// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package github implements the PR feature ports against the GitHub REST API.
package github

import (
	"context"
	"fmt"

	"github.com/task-otter/Taskotter/internal/features/pr/domain"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/githubapi"
	"github.com/task-otter/Taskotter/internal/shared/repo"
)

type (
	// Client wraps the GitHub API for TaskOtter sync operations.
	Client struct {
		api   *githubapi.Client
		owner string
		repo  string
	}
)

const (
	prTitle = "chore(taskotter): sync taskfiles"
)

// NewClient creates a GitHub API client for repository.
func NewClient(ctx context.Context, token, repository string) (*Client, error) {
	owner, repoName, err := repo.Parse(repository)
	if err != nil {
		return nil, fmt.Errorf("parse repository: %w", err)
	}

	return &Client{
		api:   githubapi.NewClient(ctx, token),
		owner: owner,
		repo:  repoName,
	}, nil
}

// CreatePR opens a new pull request from branch into base.
func (client *Client) CreatePR(
	ctx context.Context,
	req *domain.CreatePRRequest,
) (*domain.PullRequest, error) {
	pull, err := client.api.CreatePR(ctx, &githubapi.CreatePROptions{
		Owner: client.owner,
		Repo:  client.repo,
		Title: prTitle,
		Head:  req.Branch,
		Base:  req.Base,
		Body:  req.Body,
	})
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	return pullRequestFromAPI(pull), nil
}

// FindOpenPR returns an open pull request for branch into base, if one exists.
func (client *Client) FindOpenPR(
	ctx context.Context,
	branch, base string,
) (*domain.PullRequest, error) {
	pullRequests, err := client.listOpenPRsForBranch(ctx, branch, base)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}

	pr, err := openPullRequestFromList(pullRequests)
	if err != nil {
		return nil, fmt.Errorf("select open pull request: %w", err)
	}

	return pr, nil
}

// UpdatePRBody replaces the body of an existing pull request.
func (client *Client) UpdatePRBody(ctx context.Context, number int, body string) error {
	err := client.api.EditPRBody(ctx, &githubapi.EditPRBodyOptions{
		Owner:  client.owner,
		Repo:   client.repo,
		Number: number,
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("update pull request: %w", err)
	}

	return nil
}

func (client *Client) listOpenPRsForBranch(
	ctx context.Context,
	branch, base string,
) ([]githubapi.PullRequest, error) {
	pullRequests, err := client.api.ListOpenPRs(ctx, &githubapi.ListOpenPROptions{
		Owner: client.owner,
		Repo:  client.repo,
		Head:  fmt.Sprintf("%s:%s", client.owner, branch),
		Base:  base,
	})
	if err != nil {
		return nil, fmt.Errorf("list open pull requests: %w", err)
	}

	return pullRequests, nil
}

func firstOpenPullRequest(pullRequests []githubapi.PullRequest) (*domain.PullRequest, error) {
	if len(pullRequests) == consts.IndexZero {
		return nil, domain.ErrPullRequestNotFound
	}

	return pullRequestFromAPI(pullRequests[consts.IndexZero]), nil
}

func openPullRequestFromList(pullRequests []githubapi.PullRequest) (*domain.PullRequest, error) {
	pr, err := firstOpenPullRequest(pullRequests)
	if err != nil {
		return nil, fmt.Errorf("find open pull request: %w", err)
	}

	return pr, nil
}

func pullRequestFromAPI(pull githubapi.PullRequest) *domain.PullRequest {
	return &domain.PullRequest{Number: pull.Number, URL: pull.HTMLURL}
}
