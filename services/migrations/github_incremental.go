// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"time"

	base "gitea.dev/modules/migration"

	"github.com/google/go-github/v93/github"
)

// The incremental layer resumes a metadata sync from the max updated time already stored per stream.

func (g *GithubDownloaderV3) SupportSyncing() bool {
	return true
}

// GetNewIssues walks by update order, so the stored max updated time is an exact resume point
func (g *GithubDownloaderV3) GetNewIssues(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*base.Issue, bool, error) {
	return g.getIssuesSince(ctx, page, perPage, "updated", updatedAfter)
}

// GetNewPullRequests walks newest-updated first and stops at the watermark: the list has no since filter,
// and the search API is capped with a much smaller rate limit
func (g *GithubDownloaderV3) GetNewPullRequests(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*base.PullRequest, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.PullRequestListOptions{
		Sort:      "updated",
		Direction: "desc",
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}
	g.waitAndPickClient(ctx)
	prs, resp, err := g.getClient().PullRequests.List(ctx, g.repoOwner, g.repoName, opt)
	g.setRateFromResponse(resp)
	if err != nil {
		return nil, false, fmt.Errorf("GetNewPullRequests: %w", err)
	}
	allPRs := make([]*base.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if pr.GetUpdatedAt().Before(updatedAfter) {
			return allPRs, true, nil
		}
		basePR, err := g.convertGithubPullRequest(ctx, pr, perPage)
		if err != nil {
			return nil, false, err
		}
		_ = CheckAndEnsureSafePR(basePR, g.baseURL, g) // SECURITY: like the migration path
		allPRs = append(allPRs, basePR)
	}
	return allPRs, resp.NextPage == 0, nil
}

func (g *GithubDownloaderV3) GetNewComments(ctx context.Context, commentable base.Commentable, updatedAfter time.Time) ([]*base.Comment, bool, error) {
	var since *time.Time
	if !updatedAfter.IsZero() { // GitHub rejects since=0001-01-01
		since = &updatedAfter
	}
	comments, err := g.getCommentsSince(ctx, commentable, since)
	return comments, false, err
}

// GetNewReviews refetches every review: the reviews API has no since filter
func (g *GithubDownloaderV3) GetNewReviews(ctx context.Context, reviewable base.Reviewable, _ time.Time) ([]*base.Review, error) {
	return g.GetReviews(ctx, reviewable)
}

func (g *GithubGraphQLDownloader) GetNewIssues(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*base.Issue, bool, error) {
	g.since = updatedAfter
	if !g.restFallback {
		issues, isEnd, err := g.getIssuesGraphQL(ctx, page, perPage)
		if !g.fallBackToREST(ctx, err) {
			return issues, isEnd, err
		}
		g.restIssuePageOffset = page - 1
	}
	return g.GithubDownloaderV3.GetNewIssues(ctx, page-g.restIssuePageOffset, perPage, updatedAfter)
}

func (g *GithubGraphQLDownloader) GetNewPullRequests(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*base.PullRequest, bool, error) {
	g.since = updatedAfter
	if !g.restFallback {
		prs, isEnd, err := g.getPullRequestsGraphQL(ctx, page, perPage)
		if !g.fallBackToREST(ctx, err) {
			return prs, isEnd, err
		}
		g.restPRPageOffset = page - 1
	}
	return g.GithubDownloaderV3.GetNewPullRequests(ctx, page-g.restPRPageOffset, perPage, updatedAfter)
}

func (g *GithubGraphQLDownloader) GetNewComments(ctx context.Context, commentable base.Commentable, updatedAfter time.Time) ([]*base.Comment, bool, error) {
	if prefetched, ok := commentable.GetContext().(*gqlPrefetched); ok {
		return prefetched.comments, false, nil
	}
	return g.GithubDownloaderV3.GetNewComments(ctx, commentable, updatedAfter)
}

func (g *GithubGraphQLDownloader) GetNewReviews(ctx context.Context, reviewable base.Reviewable, updatedAfter time.Time) ([]*base.Review, error) {
	if prefetched, ok := reviewable.GetContext().(*gqlPrefetched); ok {
		return prefetched.reviews, nil
	}
	return g.GithubDownloaderV3.GetNewReviews(ctx, reviewable, updatedAfter)
}
