// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"testing"

	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGraphQLPullRequestParsing validates that the PR response structs match the
// shape of graphQLPullRequestsQuery and convert into the expected entities:
// pull request metadata + head/base, reviews, and inline review comments.
func TestGraphQLPullRequestParsing(t *testing.T) {
	const sample = `{
	  "repository": {
	    "pullRequests": {
	      "pageInfo": {"hasNextPage": false, "endCursor": "Y3Vyc29yOjIw"},
	      "nodes": [
	        {
	          "number": 7,
	          "title": "add feature",
	          "body": "does the thing",
	          "state": "MERGED",
	          "createdAt": "2023-05-01T00:00:00Z",
	          "updatedAt": "2023-06-01T00:00:00Z",
	          "closedAt": "2023-06-01T00:00:00Z",
	          "mergedAt": "2023-06-01T00:00:00Z",
	          "isDraft": false,
	          "locked": null,
	          "author": {"login": "contributor", "databaseId": 4242},
	          "milestone": {"title": "v2"},
	          "mergeCommit": {"oid": "abc123"},
	          "headRefName": "feature-x",
	          "headRefOid": "deadbeef",
	          "baseRefName": "main",
	          "baseRefOid": "cafebabe",
	          "headRepository": {"name": "gitea", "url": "https://github.com/contributor/gitea", "owner": {"login": "contributor"}},
	          "baseRepository": {"name": "gitea", "owner": {"login": "go-gitea"}},
	          "labels": {"nodes": [{"name": "enhancement", "color": "a2eeef", "description": ""}]},
	          "assignees": {"nodes": [{"login": "maintainer"}]},
	          "reactions": {"nodes": [{"content": "ROCKET"}]},
	          "comments": {"totalCount": 1, "nodes": [{"databaseId": 500, "body": "LGTM", "createdAt": "2023-05-02T00:00:00Z", "updatedAt": "2023-05-02T00:00:00Z", "author": {"login": "x", "databaseId": 1}, "reactions": {"nodes": []}}]},
	          "reviews": {"totalCount": 1, "nodes": [
	            {"databaseId": 900, "state": "CHANGES_REQUESTED", "body": "please fix", "createdAt": "2023-05-03T00:00:00Z", "submittedAt": "2023-05-03T01:00:00Z", "author": {"login": "reviewer", "databaseId": 77}, "commit": {"oid": "sha1sha"}, "comments": {"totalCount": 1, "nodes": [
	              {"databaseId": 950, "body": "nit here", "path": "main.go", "diffHunk": "@@ -1 +1 @@", "position": 3, "commit": {"oid": "sha1sha"}, "author": {"login": "reviewer", "databaseId": 77}, "createdAt": "2023-05-03T01:00:00Z", "updatedAt": "2023-05-03T01:00:00Z", "replyTo": null}
	            ]}}
	          ]},
	          "reviewRequests": {"nodes": [{"requestedReviewer": {"login": "pending-rev", "databaseId": 88}}]}
	        }
	      ]
	    }
	  },
	  "rateLimit": {"cost": 12, "remaining": 4988, "resetAt": "2023-01-01T00:00:00Z"}
	}`

	var resp gqlPullRequestsResponse
	require.NoError(t, json.Unmarshal([]byte(sample), &resp))
	require.Len(t, resp.Repository.PullRequests.Nodes, 1)
	assert.False(t, resp.Repository.PullRequests.PageInfo.HasNextPage)

	node := &resp.Repository.PullRequests.Nodes[0]
	g := &GithubDownloaderV3{baseURL: "https://github.com", repoOwner: "go-gitea", repoName: "gitea"}

	// PR metadata + head/base + constructed patch URL + merged/closed convention
	// (no reaction overflow in the fixture, so no sweep is triggered)
	pr, err := g.convertGraphQLPullRequest(t.Context(), node)
	require.NoError(t, err)
	assert.EqualValues(t, 7, pr.Number)
	assert.Equal(t, "closed", pr.State) // MERGED reports as closed, like REST
	assert.True(t, pr.Merged)
	assert.Equal(t, "abc123", pr.MergeCommitSHA)
	assert.Equal(t, "https://github.com/go-gitea/gitea/pull/7.patch", pr.PatchURL)
	assert.Equal(t, "feature-x", pr.Head.Ref)
	assert.Equal(t, "deadbeef", pr.Head.SHA)
	assert.Equal(t, "contributor", pr.Head.OwnerName)
	assert.Equal(t, "https://github.com/contributor/gitea.git", pr.Head.CloneURL)
	assert.Equal(t, "main", pr.Base.Ref)
	assert.Equal(t, "go-gitea", pr.Base.OwnerName)
	require.Len(t, pr.Reactions, 1)
	assert.Equal(t, "rocket", pr.Reactions[0].Content)

	// reviews: one real review (with an inline comment) + one requested-reviewer
	assert.False(t, g.reviewsOverflow(node))
	reviews := convertGraphQLReviews(node)
	require.Len(t, reviews, 2)
	assert.EqualValues(t, 900, reviews[0].ID)
	assert.Equal(t, "CHANGES_REQUESTED", reviews[0].State)
	assert.EqualValues(t, 7, reviews[0].IssueIndex)
	require.Len(t, reviews[0].Comments, 1)
	assert.EqualValues(t, 950, reviews[0].Comments[0].ID)
	assert.Equal(t, "main.go", reviews[0].Comments[0].TreePath)
	assert.Equal(t, "sha1sha", reviews[0].Comments[0].CommitID)
	// the pending review request
	assert.Equal(t, base.ReviewStateRequestReview, reviews[1].State)
	assert.Equal(t, "pending-rev", reviews[1].ReviewerName)
}

// TestGraphQLReviewsOverflow verifies the REST-fallback trigger when a PR has
// more reviews (or a review has more inline comments) than one page returned.
func TestGraphQLReviewsOverflow(t *testing.T) {
	g := &GithubDownloaderV3{}
	// reviews truncated: totalCount > returned nodes
	over := &gqlPullRequest{}
	over.Reviews.TotalCount = 60
	over.Reviews.Nodes = make([]gqlReview, 50)
	assert.True(t, g.reviewsOverflow(over))

	// a review's inline comments truncated
	inner := &gqlPullRequest{}
	inner.Reviews.TotalCount = 1
	inner.Reviews.Nodes = make([]gqlReview, 1)
	inner.Reviews.Nodes[0].Comments.TotalCount = 80
	inner.Reviews.Nodes[0].Comments.Nodes = make([]gqlReviewComment, 50)
	assert.True(t, g.reviewsOverflow(inner))

	// fully contained: no overflow
	ok := &gqlPullRequest{}
	ok.Reviews.TotalCount = 2
	ok.Reviews.Nodes = make([]gqlReview, 2)
	assert.False(t, g.reviewsOverflow(ok))
}
