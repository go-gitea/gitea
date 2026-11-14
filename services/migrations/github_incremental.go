// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/log"
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

// GetPullRequestUpdates walks newest-updated first and stops at the watermark: the list has no since filter,
// and the search API is capped with a much smaller rate limit
func (g *GithubDownloaderV3) GetPullRequestUpdates(ctx context.Context, updatedAfter time.Time) ([]base.PullRequestUpdate, error) {
	opt := &github.PullRequestListOptions{
		Sort:        "updated",
		Direction:   "desc",
		State:       "all",
		ListOptions: github.ListOptions{PerPage: g.maxPerPage},
	}
	var updates []base.PullRequestUpdate
	for page := 1; ; page++ {
		opt.Page = page
		g.waitAndPickClient(ctx)
		prs, resp, err := g.getClient().PullRequests.List(ctx, g.repoOwner, g.repoName, opt)
		g.setRateFromResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("GetPullRequestUpdates: %w", err)
		}
		for _, pr := range prs {
			if pr.GetUpdatedAt().Before(updatedAfter) {
				return updates, nil
			}
			updates = append(updates, base.PullRequestUpdate{Number: int64(pr.GetNumber()), Updated: pr.GetUpdatedAt().Time})
		}
		if resp.NextPage == 0 {
			return updates, nil
		}
	}
}

func (g *GithubDownloaderV3) GetPullRequestsByNumber(ctx context.Context, numbers []int64) ([]*base.PullRequest, error) {
	prs := make([]*base.PullRequest, 0, len(numbers))
	for _, number := range numbers {
		g.waitAndPickClient(ctx)
		pr, resp, err := g.getClient().PullRequests.Get(ctx, g.repoOwner, g.repoName, int(number))
		g.setRateFromResponse(resp)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// deleted upstream since it was listed, e.g. spam removal; failing would stop every later sync here
			log.Warn("GitHub pull request #%d in %s/%s is gone, skipped", number, g.repoOwner, g.repoName)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("GetPullRequestsByNumber: %w", err)
		}
		basePR, err := g.convertGithubPullRequest(ctx, pr, g.maxPerPage)
		if err != nil {
			return nil, err
		}
		_ = CheckAndEnsureSafePR(basePR, g.baseURL, g) // SECURITY: like the migration path
		prs = append(prs, basePR)
	}
	return prs, nil
}

// GetCommentedEntities lists the issues and pull requests with comments updated after the given time: GitHub does not always
// move an issue or pull request in its updated index for a new comment, so the issue and pull request listings miss them
func (g *GithubDownloaderV3) GetCommentedEntities(ctx context.Context, updatedAfter time.Time) ([]base.CommentedEntity, error) {
	if updatedAfter.IsZero() { // GitHub rejects since=0001-01-01, and a first sync takes everything anyway
		return nil, nil
	}
	byNumber := map[int64]*base.CommentedEntity{}
	add := func(number int64, isPull bool, updated time.Time) {
		entity, ok := byNumber[number]
		if !ok {
			byNumber[number] = &base.CommentedEntity{Number: number, IsPull: isPull, Since: updated, Until: updated}
			return
		}
		entity.IsPull = entity.IsPull || isPull
		if updated.Before(entity.Since) {
			entity.Since = updated
		}
		if updated.After(entity.Until) {
			entity.Until = updated
		}
	}

	updated, asc := "updated", "asc"
	issueOpt := &github.IssueListCommentsOptions{Sort: &updated, Direction: &asc, Since: &updatedAfter, ListOptions: github.ListOptions{PerPage: g.maxPerPage}}
	for {
		g.waitAndPickClient(ctx)
		comments, resp, err := g.getClient().Issues.ListComments(ctx, g.repoOwner, g.repoName, 0, issueOpt)
		g.setRateFromResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("GetCommentedEntities: %w", err)
		}
		for _, comment := range comments {
			if number, ok := numberFromURL(comment.GetIssueURL()); ok {
				// the issue url is the same for pull requests, the html one is not
				add(number, strings.Contains(comment.GetHTMLURL(), "/pull/"), comment.GetUpdatedAt().Time)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		issueOpt.Page = resp.NextPage
	}

	reviewOpt := &github.PullRequestListCommentsOptions{Sort: updated, Direction: asc, Since: updatedAfter, ListOptions: github.ListOptions{PerPage: g.maxPerPage}}
	for {
		g.waitAndPickClient(ctx)
		comments, resp, err := g.getClient().PullRequests.ListComments(ctx, g.repoOwner, g.repoName, 0, reviewOpt)
		g.setRateFromResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("GetCommentedEntities: %w", err)
		}
		for _, comment := range comments {
			if number, ok := numberFromURL(comment.GetPullRequestURL()); ok {
				add(number, true, comment.GetUpdatedAt().Time)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		reviewOpt.Page = resp.NextPage
	}

	entities := make([]base.CommentedEntity, 0, len(byNumber))
	for _, entity := range byNumber {
		entities = append(entities, *entity)
	}
	slices.SortFunc(entities, func(a, b base.CommentedEntity) int { return a.Until.Compare(b.Until) })
	return entities, nil
}

func numberFromURL(u string) (int64, bool) {
	number, err := strconv.ParseInt(path.Base(u), 10, 64)
	return number, err == nil && number > 0
}

func (g *GithubDownloaderV3) GetIssuesByNumber(ctx context.Context, numbers []int64) ([]*base.Issue, error) {
	issues := make([]*base.Issue, 0, len(numbers))
	for _, number := range numbers {
		g.waitAndPickClient(ctx)
		issue, resp, err := g.getClient().Issues.Get(ctx, g.repoOwner, g.repoName, int(number))
		g.setRateFromResponse(resp)
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			log.Warn("GitHub issue #%d in %s/%s is gone, skipped", number, g.repoOwner, g.repoName)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("GetIssuesByNumber: %w", err)
		}
		if issue.IsPullRequest() {
			continue
		}
		converted, err := g.convertGithubIssue(ctx, issue, g.maxPerPage)
		if err != nil {
			return nil, err
		}
		issues = append(issues, converted)
	}
	return issues, nil
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

func (g *GithubGraphQLDownloader) GetPullRequestUpdates(ctx context.Context, updatedAfter time.Time) ([]base.PullRequestUpdate, error) {
	if !g.restFallback {
		updates, err := g.getPullRequestUpdatesGraphQL(ctx, updatedAfter)
		if !g.fallBackToREST(ctx, err) {
			return updates, err
		}
	}
	return g.GithubDownloaderV3.GetPullRequestUpdates(ctx, updatedAfter)
}

func (g *GithubGraphQLDownloader) GetPullRequestsByNumber(ctx context.Context, numbers []int64) ([]*base.PullRequest, error) {
	if !g.restFallback {
		prs, err := g.getPullRequestsByNumberGraphQL(ctx, numbers)
		if !g.fallBackToREST(ctx, err) {
			return prs, err
		}
	}
	return g.GithubDownloaderV3.GetPullRequestsByNumber(ctx, numbers)
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
