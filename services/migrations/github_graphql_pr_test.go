// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphQLPullRequestParsing(t *testing.T) {
	var response gqlPullRequestsResponse
	require.NoError(t, json.Unmarshal([]byte(`{"repository":{"pullRequests":{"nodes":[{
		"number":7,"title":"add feature","body":"does the thing","state":"MERGED",
		"createdAt":"2023-05-01T00:00:00Z","updatedAt":"2023-06-01T00:00:00Z",
		"closedAt":"2023-06-01T00:00:00Z","mergedAt":"2023-06-01T00:00:00Z",
		"author":{"login":"contributor","databaseId":4242},"isDraft":true,
		"mergeCommit":{"oid":"abc123"},"headRefName":"feature-x","headRefOid":"deadbeef",
		"baseRefName":"main","baseRefOid":"cafebabe",
		"headRepository":{"name":"gitea","url":"https://github.com/contributor/gitea"},
		"headRepositoryOwner":{"login":"contributor"},
		"baseRepository":{"name":"gitea","owner":{"login":"go-gitea"}},
		"reviews":{"nodes":[{
			"databaseId":900,"state":"CHANGES_REQUESTED","body":"please fix",
			"createdAt":"2023-05-03T00:00:00Z","submittedAt":"2023-05-03T01:00:00Z",
			"author":{"login":"reviewer","databaseId":77},"commit":{"oid":"sha1sha"},
			"comments":{"nodes":[{"databaseId":950,"body":"nit here","path":"main.go","diffHunk":"@@ -1 +1 @@",
				"position":3,"commit":{"oid":"sha1sha"},"author":{"databaseId":77},"replyTo":{"databaseId":949}}]}
		}]},
		"reviewRequests":{"nodes":[{"requestedReviewer":{"__typename":"Bot","login":"pending-rev","databaseId":88}},
			{"requestedReviewer":{"__typename":"Team"}},{"requestedReviewer":null}]}
	}]}}}`), &response))
	downloader := &GithubGraphQLDownloader{GithubDownloaderV3: &GithubDownloaderV3{baseURL: "https://github.com", repoOwner: "go-gitea", repoName: "gitea"}}
	pr, err := downloader.convertGraphQLPullRequest(t.Context(), &response.Repository.PullRequests.Nodes[0], map[string]*gqlPrefetched{})
	require.NoError(t, err)
	merged := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, &base.PullRequest{
		Number: 7, Title: "add feature", Content: "does the thing", State: "closed", PosterID: 4242, PosterName: "contributor",
		Created: time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC), Updated: merged, Closed: &merged,
		Merged: true, MergedTime: &merged, IsDraft: true, MergeCommitSHA: "abc123", ForeignIndex: 7,
		Head:     base.PullRequestBranch{Ref: "feature-x", SHA: "deadbeef", OwnerName: "contributor", RepoName: "gitea", CloneURL: "https://github.com/contributor/gitea.git"},
		Base:     base.PullRequestBranch{Ref: "main", SHA: "cafebabe", OwnerName: "go-gitea", RepoName: "gitea"},
		PatchURL: "https://github.com/go-gitea/gitea/pull/7.patch", Labels: []*base.Label{}, Context: pr.Context,
	}, pr)
	reviews, err := downloader.GetReviews(t.Context(), pr)
	require.NoError(t, err)
	assert.Equal(t, []*base.Review{
		{
			ID: 900, IssueIndex: 7, ReviewerID: 77, ReviewerName: "reviewer", Content: "please fix", State: "CHANGES_REQUESTED", CommitID: "sha1sha",
			CreatedAt: time.Date(2023, 5, 3, 1, 0, 0, 0, time.UTC),
			Comments:  []*base.ReviewComment{{ID: 950, InReplyTo: 949, Content: "nit here", TreePath: "main.go", DiffHunk: "@@ -1 +1 @@", Position: 3, CommitID: "sha1sha", PosterID: 77}},
		},
		{IssueIndex: 7, ReviewerID: 88, ReviewerName: "pending-rev[bot]", State: base.ReviewStateRequestReview},
	}, reviews)
}

func TestGraphQLDiscussionOverflowFallsBackToREST(t *testing.T) {
	for _, connection := range []string{"comments", "reviews", "review comments"} {
		t.Run(connection, func(t *testing.T) {
			downloader := newGithubTestDownloader(t, func(writer http.ResponseWriter, request *http.Request) {
				switch {
				case strings.HasSuffix(request.URL.Path, "/issues/7/comments"):
					_, _ = io.WriteString(writer, `[{"id":500,"body":"REST comment"}]`)
				case strings.HasSuffix(request.URL.Path, "/pulls/7/reviews"):
					_, _ = io.WriteString(writer, `[{"id":900,"body":"REST review","state":"APPROVED"}]`)
				case strings.HasSuffix(request.URL.Path, "/requested_reviewers"):
					_, _ = io.WriteString(writer, `{}`)
				default:
					_, _ = io.WriteString(writer, `[]`)
				}
			})
			downloader.SkipReactions = true
			node := &gqlPullRequest{gqlIssue: gqlIssue{Number: 7}}
			switch connection {
			case "comments":
				node.Comments.PageInfo.HasNextPage = true
			case "reviews":
				node.Reviews.PageInfo.HasNextPage = true
			case "review comments":
				node.Reviews.Nodes = []gqlReview{{Comments: gqlConn[gqlReviewComment]{PageInfo: gqlPageInfo{HasNextPage: true}}}}
			}
			pr, err := downloader.convertGraphQLPullRequest(t.Context(), node, map[string]*gqlPrefetched{})
			require.NoError(t, err)
			if connection == "comments" {
				comments, _, err := downloader.GetComments(t.Context(), pr)
				require.NoError(t, err)
				assert.Equal(t, []*base.Comment{{IssueIndex: 7, Index: 500, Content: "REST comment"}}, comments)
			} else {
				reviews, err := downloader.GetReviews(t.Context(), pr)
				require.NoError(t, err)
				assert.Equal(t, []*base.Review{{IssueIndex: 7, ID: 900, Content: "REST review", State: "APPROVED"}}, reviews)
			}
		})
	}
}
