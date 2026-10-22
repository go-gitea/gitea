// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"slices"
	"time"

	base "gitea.dev/modules/migration"
)

const graphQLPRFields = "mergedAt isDraft mergeCommit{oid} headRefName headRefOid baseRefName baseRefOid headRepository{name url} headRepositoryOwner{login} baseRepository{name owner{login}}"

var graphQLPRDiscussion = fmt.Sprintf(`
        reviews(first:50){%[1]s nodes{
          databaseId state body createdAt submittedAt author{%[2]s} commit{oid}
          comments(first:50){%[1]s nodes{databaseId body path diffHunk position commit{oid} author{%[2]s} createdAt updatedAt replyTo{databaseId}}}
        }}
        reviewRequests(first:%[3]d){%[1]s nodes{requestedReviewer{%[4]s}}}`,
	gqlPageInfoFields, gqlActorFields, graphQLReviewRequestPageSize, gqlReviewerFields)

var graphQLReviewRequestsSweepQuery = nodeConnQuery("PullRequest", "reviewRequests", "requestedReviewer{"+gqlReviewerFields+"}")

func (g *GithubGraphQLDownloader) graphQLPullRequestsQuery() string {
	return g.graphQLContentQuery("pullRequests", graphQLPRFields, graphQLPRDiscussion)
}

type gqlRepoRef struct {
	Name  string   `json:"name"`
	URL   string   `json:"url"`
	Owner gqlActor `json:"owner"`
}

type gqlReviewComment struct {
	DatabaseID int64      `json:"databaseId"`
	Body       string     `json:"body"`
	Path       string     `json:"path"`
	DiffHunk   string     `json:"diffHunk"`
	Position   int        `json:"position"`
	Commit     *gqlCommit `json:"commit"`
	Author     gqlActor   `json:"author"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	ReplyTo    *struct {
		DatabaseID int64 `json:"databaseId"`
	} `json:"replyTo"`
}

type gqlReview struct {
	DatabaseID  int64                     `json:"databaseId"`
	State       string                    `json:"state"`
	Body        string                    `json:"body"`
	CreatedAt   time.Time                 `json:"createdAt"`
	SubmittedAt *time.Time                `json:"submittedAt"`
	Author      gqlActor                  `json:"author"`
	Commit      *gqlCommit                `json:"commit"`
	Comments    gqlConn[gqlReviewComment] `json:"comments"`
}

type gqlReviewRequest struct {
	RequestedReviewer *gqlActor `json:"requestedReviewer"`
}

type gqlPullRequest struct {
	gqlIssue
	MergedAt       *time.Time                `json:"mergedAt"`
	IsDraft        bool                      `json:"isDraft"`
	MergeCommit    *gqlCommit                `json:"mergeCommit"`
	HeadRefName    string                    `json:"headRefName"`
	HeadRefOID     string                    `json:"headRefOid"`
	BaseRefName    string                    `json:"baseRefName"`
	BaseRefOID     string                    `json:"baseRefOid"`
	HeadRepo       *gqlRepoRef               `json:"headRepository"`
	HeadRepoOwner  *gqlActor                 `json:"headRepositoryOwner"`
	BaseRepo       *gqlRepoRef               `json:"baseRepository"`
	Reviews        gqlConn[gqlReview]        `json:"reviews"`
	ReviewRequests gqlConn[gqlReviewRequest] `json:"reviewRequests"`
}

type gqlPullRequestsResponse struct {
	Repository struct {
		PullRequests gqlConn[gqlPullRequest] `json:"pullRequests"`
	} `json:"repository"`
}

func (g *GithubGraphQLDownloader) getPullRequestsGraphQL(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	if page <= 1 {
		g.prCursor = ""
	}
	var resp gqlPullRequestsResponse
	if err := g.doGraphQLPage(ctx, g.graphQLPullRequestsQuery(), g.prCursor, graphQLPageSize(perPage, githubGraphQLPRPageSize), &resp); err != nil {
		return nil, false, err
	}
	conn := resp.Repository.PullRequests
	prefetched := map[string]*gqlPrefetched{}
	prs := make([]*base.PullRequest, 0, len(conn.Nodes))
	for i := range conn.Nodes {
		pr, err := g.convertGraphQLPullRequest(ctx, &conn.Nodes[i], prefetched)
		if err != nil {
			return nil, false, err
		}
		_ = CheckAndEnsureSafePR(pr, g.baseURL, g) // SECURITY: like the REST path
		prs = append(prs, pr)
	}
	if err := g.enrichGraphQLPage(ctx, prefetched); err != nil {
		return nil, false, err
	}
	g.prCursor = conn.PageInfo.EndCursor // only now, a failed page is retried from the same cursor
	return prs, !conn.PageInfo.HasNextPage, nil
}

func (g *GithubGraphQLDownloader) convertGraphQLPullRequest(ctx context.Context, node *gqlPullRequest, prefetchedByNodeID map[string]*gqlPrefetched) (*base.PullRequest, error) {
	issue, err := g.convertGraphQLIssue(ctx, &node.gqlIssue, prefetchedByNodeID)
	if err != nil {
		return nil, err
	}
	if prefetched, ok := issue.Context.(*gqlPrefetched); ok {
		if prefetched.reviews, err = g.prefetchReviews(ctx, node); err != nil {
			return nil, err
		}
	}
	pr := &base.PullRequest{
		Number:       issue.Number,
		Title:        issue.Title,
		PosterID:     issue.PosterID,
		PosterName:   issue.PosterName,
		Content:      issue.Content,
		Milestone:    issue.Milestone,
		State:        "closed", // REST reports a merged PR as closed
		Created:      issue.Created,
		Updated:      issue.Updated,
		Closed:       issue.Closed,
		Labels:       issue.Labels,
		Assignees:    issue.Assignees,
		IsLocked:     issue.IsLocked,
		Reactions:    issue.Reactions,
		ForeignIndex: issue.ForeignIndex,
		Context:      issue.Context,
		PatchURL:     fmt.Sprintf("%s/%s/%s/pull/%d.patch", g.baseURL, g.repoOwner, g.repoName, node.Number),
		Merged:       node.MergedAt != nil || node.State == "MERGED",
		MergedTime:   node.MergedAt,
		IsDraft:      node.IsDraft,
		Head:         base.PullRequestBranch{Ref: node.HeadRefName, SHA: node.HeadRefOID},
		Base:         base.PullRequestBranch{Ref: node.BaseRefName, SHA: node.BaseRefOID},
	}
	if node.State == "OPEN" {
		pr.State = "open"
	}
	if node.MergeCommit != nil {
		pr.MergeCommitSHA = node.MergeCommit.OID
	}
	if node.HeadRepoOwner != nil {
		pr.Head.OwnerName = node.HeadRepoOwner.Login
	}
	if node.HeadRepo != nil {
		pr.Head.RepoName = node.HeadRepo.Name
		pr.Head.CloneURL = node.HeadRepo.URL + ".git"
	}
	if node.BaseRepo != nil {
		pr.Base.RepoName = node.BaseRepo.Name
		pr.Base.OwnerName = node.BaseRepo.Owner.Login
	}
	return pr, nil
}

func (g *GithubGraphQLDownloader) prefetchReviews(ctx context.Context, node *gqlPullRequest) ([]*base.Review, error) {
	if node.Reviews.PageInfo.HasNextPage || slices.ContainsFunc(node.Reviews.Nodes, func(r gqlReview) bool { return r.Comments.PageInfo.HasNextPage }) {
		return g.GetReviews(ctx, &base.PullRequest{Number: node.Number, ForeignIndex: node.Number})
	}
	requests, err := allNodes(ctx, g, graphQLReviewRequestsSweepQuery, node.ID, node.ReviewRequests)
	if err != nil {
		return nil, err
	}
	return convertGraphQLReviews(node, requests), nil
}

func convertGraphQLReviews(node *gqlPullRequest, requests []gqlReviewRequest) []*base.Review {
	reviews := make([]*base.Review, 0, len(node.Reviews.Nodes)+len(requests))
	for _, sourceReview := range node.Reviews.Nodes {
		review := &base.Review{
			ID:           sourceReview.DatabaseID,
			IssueIndex:   node.Number,
			ReviewerID:   sourceReview.Author.DatabaseID,
			ReviewerName: sourceReview.Author.Login,
			Content:      sourceReview.Body,
			CreatedAt:    sourceReview.CreatedAt,
			State:        sourceReview.State,
		}
		if sourceReview.SubmittedAt != nil {
			review.CreatedAt = *sourceReview.SubmittedAt
		}
		if sourceReview.Commit != nil {
			review.CommitID = sourceReview.Commit.OID
		}
		for _, sourceComment := range sourceReview.Comments.Nodes {
			comment := &base.ReviewComment{
				ID:        sourceComment.DatabaseID,
				Content:   sourceComment.Body,
				TreePath:  sourceComment.Path,
				DiffHunk:  sourceComment.DiffHunk,
				Position:  sourceComment.Position,
				PosterID:  sourceComment.Author.DatabaseID,
				CreatedAt: sourceComment.CreatedAt,
				UpdatedAt: sourceComment.UpdatedAt,
			}
			if sourceComment.ReplyTo != nil {
				comment.InReplyTo = sourceComment.ReplyTo.DatabaseID
			}
			if sourceComment.Commit != nil {
				comment.CommitID = sourceComment.Commit.OID
			}
			review.Comments = append(review.Comments, comment)
		}
		reviews = append(reviews, review)
	}
	for _, request := range requests {
		if reviewer := request.RequestedReviewer; reviewer != nil && reviewer.Login != "" {
			reviews = append(reviews, &base.Review{
				IssueIndex:   node.Number,
				ReviewerID:   reviewer.DatabaseID,
				ReviewerName: reviewer.Login,
				State:        base.ReviewStateRequestReview,
			})
		}
	}
	return reviews
}
