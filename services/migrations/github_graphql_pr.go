// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"time"

	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
)

// graphQLPullRequestsQuery keeps comment reactions out for the 500,000-node limit, like graphQLIssuesQuery;
// review-comment reactions are not fetched yet. Paged by createdAt since a cursor over a mutable sort key loses rows.
func (g *GithubDownloaderV3) graphQLPullRequestsQuery() string {
	if g.gqlPullRequestsQuery == "" {
		prReactions := ""
		if !g.SkipReactions {
			prReactions = "reactions(first:100){totalCount nodes{" + gqlReactionFields + "}}"
		}
		discussion := ""
		if !g.gqlSkipComments {
			discussion = fmt.Sprintf(`comments(first:100){totalCount nodes{id databaseId body createdAt updatedAt author{%[1]s}}}
        reviews(first:50){totalCount nodes{
          databaseId state body createdAt submittedAt
          author{%[1]s}
          commit{oid}
          comments(first:50){totalCount nodes{databaseId body path diffHunk position commit{oid} author{%[1]s} createdAt updatedAt replyTo{databaseId}}}
        }}
        reviewRequests(first:%[2]d){totalCount nodes{requestedReviewer{... on User{login databaseId} ... on Mannequin{login databaseId}}}}`,
				gqlActorFields, graphQLReviewRequestPageSize)
		}
		g.gqlPullRequestsQuery = fmt.Sprintf(`
query($owner:String!,$name:String!,$cursor:String,$first:Int!){
  repository(owner:$owner,name:$name){
    pullRequests(first:$first,after:$cursor,orderBy:{field:CREATED_AT,direction:ASC},states:[OPEN,CLOSED,MERGED]){
      pageInfo{hasNextPage endCursor}
      nodes{
        id number title body state createdAt updatedAt closedAt mergedAt isDraft
        locked
        author{%[1]s}
        milestone{title}
        mergeCommit{oid}
        headRefName headRefOid baseRefName baseRefOid
        headRepository{name url owner{login}}
        baseRepository{name owner{login}}
        labels(first:%[2]d){totalCount nodes{name color description}}
        assignees(first:%[3]d){totalCount nodes{login}}
        %[4]s
        %[5]s
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`, gqlActorFields, graphQLLabelPageSize, graphQLAssigneePageSize, prReactions, discussion)
	}
	return g.gqlPullRequestsQuery
}

type gqlRepoRef struct {
	Name  string   `json:"name"`
	URL   string   `json:"url"`
	Owner gqlActor `json:"owner"`
}

type gqlReviewComment struct {
	DatabaseID int64  `json:"databaseId"`
	Body       string `json:"body"`
	Path       string `json:"path"`
	DiffHunk   string `json:"diffHunk"`
	Position   int    `json:"position"`
	Commit     *struct {
		OID string `json:"oid"`
	} `json:"commit"`
	Author    gqlActor  `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	ReplyTo   *struct {
		DatabaseID int64 `json:"databaseId"`
	} `json:"replyTo"`
}

type gqlReview struct {
	DatabaseID  int64      `json:"databaseId"`
	State       string     `json:"state"`
	Body        string     `json:"body"`
	CreatedAt   time.Time  `json:"createdAt"`
	SubmittedAt *time.Time `json:"submittedAt"`
	Author      gqlActor   `json:"author"`
	Commit      *struct {
		OID string `json:"oid"`
	} `json:"commit"`
	Comments struct {
		TotalCount int                `json:"totalCount"`
		Nodes      []gqlReviewComment `json:"nodes"`
	} `json:"comments"`
}

type gqlPullRequest struct {
	ID        string     `json:"id"`
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"` // OPEN, CLOSED, MERGED
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	MergedAt  *time.Time `json:"mergedAt"`
	IsDraft   bool       `json:"isDraft"`
	Locked    bool       `json:"locked"` // activeLockReason is null when locked without a reason
	Author    gqlActor   `json:"author"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	MergeCommit *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
	HeadRefName string          `json:"headRefName"`
	HeadRefOID  string          `json:"headRefOid"`
	BaseRefName string          `json:"baseRefName"`
	BaseRefOID  string          `json:"baseRefOid"`
	HeadRepo    *gqlRepoRef     `json:"headRepository"`
	BaseRepo    *gqlRepoRef     `json:"baseRepository"`
	Labels      gqlLabelConn    `json:"labels"`
	Assignees   gqlAssigneeConn `json:"assignees"`
	Reactions   gqlReactionConn `json:"reactions"`
	Comments    struct {
		TotalCount int          `json:"totalCount"`
		Nodes      []gqlComment `json:"nodes"`
	} `json:"comments"`
	Reviews struct {
		TotalCount int         `json:"totalCount"`
		Nodes      []gqlReview `json:"nodes"`
	} `json:"reviews"`
	ReviewRequests gqlReviewRequestConn `json:"reviewRequests"`
}

type gqlReviewRequestNode struct {
	RequestedReviewer gqlActor `json:"requestedReviewer"`
}

type gqlReviewRequestConn struct {
	TotalCount int                    `json:"totalCount"`
	Nodes      []gqlReviewRequestNode `json:"nodes"`
}

type gqlPullRequestsResponse struct {
	Repository struct {
		PullRequests struct {
			PageInfo gqlPageInfo      `json:"pageInfo"`
			Nodes    []gqlPullRequest `json:"nodes"`
		} `json:"pullRequests"`
	} `json:"repository"`
	RateLimit graphQLRateLimit `json:"rateLimit"`
}

// getPullRequestsGraphQL fetches a page of pull requests with their comments and reviews, caching both for later phases.
func (g *GithubDownloaderV3) getPullRequestsGraphQL(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	if page <= 1 {
		g.gqlPRCursor = ""
		log.Info("metadata sync [%s/%s]: pull requests — full sweep", g.repoOwner, g.repoName)
	}
	g.gqlComments = map[int64][]*base.Comment{}
	g.gqlReviews = map[int64][]*base.Review{}

	vars := map[string]any{"owner": g.repoOwner, "name": g.repoName}
	if g.gqlPRCursor != "" {
		vars["cursor"] = g.gqlPRCursor
	}

	var resp gqlPullRequestsResponse
	if err := g.doGraphQLPageShrink(ctx, g.graphQLPullRequestsQuery(), vars, &resp, graphQLPageSize(perPage, githubGraphQLPRPageSize), "pull requests"); err != nil {
		return nil, false, err
	}
	g.recordGraphQLBudget(resp.RateLimit)

	allPRs := make([]*base.PullRequest, 0, len(resp.Repository.PullRequests.Nodes))
	timelineTargets := map[string]int64{}
	commentReactionTargets := map[string]*base.Comment{}
	for i := range resp.Repository.PullRequests.Nodes {
		node := &resp.Repository.PullRequests.Nodes[i]

		pr, err := g.convertGraphQLPullRequest(ctx, node)
		if err != nil {
			return nil, false, err
		}
		allPRs = append(allPRs, pr)
		// SECURITY: ensure the PR is safe (mirrors the REST path)
		_ = CheckAndEnsureSafePR(pr, g.baseURL, g)
		if g.gqlSkipComments {
			continue
		}
		timelineTargets[node.ID] = node.Number

		if node.Comments.TotalCount > len(node.Comments.Nodes) {
			rest, err := g.getComments(ctx, pr)
			if err != nil {
				return nil, false, err
			}
			g.gqlComments[node.Number] = rest
		} else {
			cached := convertGraphQLComments(node.Number, node.Comments.Nodes)
			g.gqlComments[node.Number] = cached
			for j := range cached {
				commentReactionTargets[node.Comments.Nodes[j].ID] = cached[j]
			}
		}

		if g.reviewsOverflow(node) {
			restReviews, err := g.getReviewsREST(ctx, pr)
			if err != nil {
				return nil, false, err
			}
			g.gqlReviews[node.Number] = restReviews
		} else {
			reviews, err := g.reviewsWithRequestSweep(ctx, node)
			if err != nil {
				return nil, false, err
			}
			g.gqlReviews[node.Number] = reviews
		}
	}

	// only after the page converted: a failed page is retried with the same page number
	g.gqlPRCursor = resp.Repository.PullRequests.PageInfo.EndCursor

	log.Info("metadata sync [%s/%s]: PRs page %d — %d pull requests, %d timeline targets",
		g.repoOwner, g.repoName, page, len(allPRs), len(timelineTargets))

	// best-effort enrichments that must not abort the import
	if err := g.attachCommentReactions(ctx, commentReactionTargets); err != nil {
		if ctx.Err() != nil {
			return nil, false, err
		}
		log.Error("github graphql: PR comment reactions sync failed, importing without them: %v", err)
	}
	if err := g.attachTimelineEvents(ctx, timelineTargets); err != nil {
		if ctx.Err() != nil {
			return nil, false, err
		}
		log.Error("github graphql: PR timeline events sync failed, importing without them: %v", err)
	}

	return allPRs, !resp.Repository.PullRequests.PageInfo.HasNextPage, nil
}

func (g *GithubDownloaderV3) reviewsOverflow(node *gqlPullRequest) bool {
	if node.Reviews.TotalCount > len(node.Reviews.Nodes) {
		return true
	}
	for i := range node.Reviews.Nodes {
		if node.Reviews.Nodes[i].Comments.TotalCount > len(node.Reviews.Nodes[i].Comments.Nodes) {
			return true
		}
	}
	return false
}

func (g *GithubDownloaderV3) convertGraphQLPullRequest(ctx context.Context, node *gqlPullRequest) (*base.PullRequest, error) {
	prReactions, err := g.reactionsWithSweep(ctx, node.ID, node.Reactions)
	if err != nil {
		return nil, err
	}
	labels, err := g.labelsWithSweep(ctx, node.ID, node.Labels)
	if err != nil {
		return nil, err
	}
	assignees, err := g.assigneesWithSweep(ctx, node.ID, node.Assignees)
	if err != nil {
		return nil, err
	}
	var milestone string
	if node.Milestone != nil {
		milestone = node.Milestone.Title
	}
	var mergeCommitSHA string
	if node.MergeCommit != nil {
		mergeCommitSHA = node.MergeCommit.OID
	}

	head := base.PullRequestBranch{Ref: node.HeadRefName, SHA: node.HeadRefOID}
	if node.HeadRepo != nil {
		head.RepoName = node.HeadRepo.Name
		head.OwnerName = node.HeadRepo.Owner.Login
		head.CloneURL = node.HeadRepo.URL + ".git"
	}
	baseBranch := base.PullRequestBranch{Ref: node.BaseRefName, SHA: node.BaseRefOID}
	if node.BaseRepo != nil {
		baseBranch.RepoName = node.BaseRepo.Name
		baseBranch.OwnerName = node.BaseRepo.Owner.Login
	}

	// GitHub REST reports a merged PR as state "closed"; keep that convention.
	state := "open"
	if node.State != "OPEN" {
		state = "closed"
	}

	return &base.PullRequest{
		Number:         node.Number,
		Title:          node.Title,
		PosterID:       node.Author.orGhost().DatabaseID,
		PosterName:     node.Author.orGhost().Login,
		Content:        node.Body,
		Milestone:      milestone,
		State:          state,
		Created:        node.CreatedAt,
		Updated:        node.UpdatedAt,
		Closed:         node.ClosedAt,
		Labels:         labels,
		PatchURL:       fmt.Sprintf("%s/%s/%s/pull/%d.patch", g.baseURL, g.repoOwner, g.repoName, node.Number),
		Merged:         node.MergedAt != nil || node.State == "MERGED",
		MergedTime:     node.MergedAt,
		MergeCommitSHA: mergeCommitSHA,
		Head:           head,
		Base:           baseBranch,
		Assignees:      assignees,
		IsLocked:       node.Locked,
		Reactions:      prReactions,
		ForeignIndex:   node.Number,
		IsDraft:        node.IsDraft,
	}, nil
}

const graphQLReviewRequestsSweepQuery = `
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on PullRequest{
      conn:reviewRequests(first:100,after:$cursor){
        pageInfo{hasNextPage endCursor}
        nodes{requestedReviewer{... on User{login databaseId} ... on Mannequin{login databaseId}}}
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`

func (g *GithubDownloaderV3) reviewsWithRequestSweep(ctx context.Context, node *gqlPullRequest) ([]*base.Review, error) {
	requests := node.ReviewRequests.Nodes
	if node.ReviewRequests.TotalCount > len(requests) && node.ID != "" {
		swept, err := sweepNodeConnection[gqlReviewRequestNode](ctx, g, graphQLReviewRequestsSweepQuery, node.ID, "")
		if err != nil {
			return nil, err
		}
		requests = swept
	}
	return convertGraphQLReviews(node, requests), nil
}

func convertGraphQLReviews(node *gqlPullRequest, requests []gqlReviewRequestNode) []*base.Review {
	reviews := make([]*base.Review, 0, len(node.Reviews.Nodes)+len(requests))
	for i := range node.Reviews.Nodes {
		r := &node.Reviews.Nodes[i]
		created := r.CreatedAt
		if r.SubmittedAt != nil {
			created = *r.SubmittedAt
		}
		var commitID string
		if r.Commit != nil {
			commitID = r.Commit.OID
		}
		reviews = append(reviews, &base.Review{
			ID:           r.DatabaseID,
			IssueIndex:   node.Number,
			ReviewerID:   r.Author.orGhost().DatabaseID,
			ReviewerName: r.Author.orGhost().Login,
			CommitID:     commitID,
			Content:      r.Body,
			CreatedAt:    created,
			State:        r.State, // GraphQL and REST share the review-state enum
			Comments:     convertGraphQLReviewComments(r),
		})
	}
	for _, rr := range requests {
		if rr.RequestedReviewer.Login == "" {
			continue // non-user reviewer (team) — TODO
		}
		reviews = append(reviews, &base.Review{
			IssueIndex:   node.Number,
			ReviewerID:   rr.RequestedReviewer.DatabaseID,
			ReviewerName: rr.RequestedReviewer.Login,
			State:        base.ReviewStateRequestReview,
		})
	}
	return reviews
}

func convertGraphQLReviewComments(r *gqlReview) []*base.ReviewComment {
	comments := make([]*base.ReviewComment, 0, len(r.Comments.Nodes))
	for i := range r.Comments.Nodes {
		c := &r.Comments.Nodes[i]
		var inReplyTo int64
		if c.ReplyTo != nil {
			inReplyTo = c.ReplyTo.DatabaseID
		}
		var commitID string
		if c.Commit != nil {
			commitID = c.Commit.OID
		}
		comments = append(comments, &base.ReviewComment{
			ID:        c.DatabaseID,
			InReplyTo: inReplyTo,
			Content:   c.Body,
			TreePath:  c.Path,
			DiffHunk:  c.DiffHunk,
			Position:  c.Position,
			CommitID:  commitID,
			PosterID:  c.Author.orGhost().DatabaseID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}
	return comments
}
