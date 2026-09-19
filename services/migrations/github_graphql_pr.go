// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

// GraphQL fast path for pull requests — the follow-on to the issues path in
// github_graphql.go. It fetches a pull request together with its comments,
// reviews and review (inline) comments in a single batched request instead of
// the REST path's per-PR ListReviews + per-review ListReviewComments N+1.
//
// Reviews are cached so the framework's separate review phase serves them from
// memory; a PR's issue-comments join the shared comment cache so GetComments
// serves issue and PR comments alike. The patch URL is constructed (a
// raw download, not an API call), so no API budget is spent resolving it.
// Entities that exceed one page (issue comments, reviews, or a review's inline
// comments) fall back to REST for that pull request so nothing is dropped.

import (
	"context"
	"fmt"
	"time"

	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
)

// graphQLPullRequestsQuery builds the PR query. Same 500k-node ceiling and
// per-query compute budget as the issues query: nesting reactions under comments
// (pullRequests×comments×reactions) is a 3rd connection level that blows both,
// so the content query carries only the PR-level reactions; PR-comment reactions
// come from the batched node-id pass (attachCommentReactions). Review-comment
// reactions are deliberately NOT fetched: that path is four deep
// (pullRequests×reviews×comments×reactions) and needs a separate pass — a
// follow-up. The dominant node path stays reviews(50)×comments(50) = 50k, far
// under the ceiling.
//
// Pull requests are paged oldest-created-first, like the issue stream: cursor
// pagination over UPDATED_AT would lose any pull request touched mid-sweep, because
// a new comment re-sorts it behind the cursor where no later page ever returns it.
func (g *GithubDownloaderV3) graphQLPullRequestsQuery() string {
	if g.gqlPullRequestsQuery == "" {
		prReactions := ""
		if !g.SkipReactions {
			prReactions = "reactions(first:100){totalCount nodes{" + gqlReactionFields + "}}"
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
        %[5]s
        comments(first:100){totalCount nodes{id databaseId body createdAt updatedAt author{%[1]s}}}
        reviews(first:50){totalCount nodes{
          databaseId state body createdAt submittedAt
          author{%[1]s}
          commit{oid}
          comments(first:50){totalCount nodes{databaseId body path diffHunk position commit{oid} author{%[1]s} createdAt updatedAt replyTo{databaseId}}}
        }}
        reviewRequests(first:%[4]d){totalCount nodes{requestedReviewer{... on User{login databaseId} ... on Mannequin{login databaseId}}}}
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`, gqlActorFields, graphQLLabelPageSize, graphQLAssigneePageSize, graphQLReviewRequestPageSize, prReactions)
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
	ID        string     `json:"id"` // GraphQL node id, for the reactions sweep and timeline pass
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"` // OPEN, CLOSED, MERGED
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	MergedAt  *time.Time `json:"mergedAt"`
	IsDraft   bool       `json:"isDraft"`
	Locked    bool       `json:"locked"` // the real locked boolean (activeLockReason is null when locked without a reason)
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

// gqlReviewRequestConn carries totalCount so a pull request with more pending
// reviewers than the query's page size is swept rather than truncated.
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

// getPullRequestsGraphQL fetches a page of pull requests (oldest created first)
// together with their comments, reviews and review comments in one request.
// Comments are cached alongside the issue comments and reviews are cached per PR
// for the framework's later phases. page<=1 marks the first request of a sweep.
func (g *GithubDownloaderV3) getPullRequestsGraphQL(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	if page <= 1 {
		g.gqlPRCursor = ""
		g.gqlReviews = map[int64][]*base.Review{}
		log.Info("metadata sync [%s/%s]: pull requests — full sweep", g.repoOwner, g.repoName)
	}
	if g.gqlComments == nil {
		g.gqlComments = map[int64][]*base.Comment{}
	}

	vars := map[string]any{"owner": g.repoOwner, "name": g.repoName}
	if g.gqlPRCursor != "" {
		vars["cursor"] = g.gqlPRCursor
	}

	var resp gqlPullRequestsResponse
	if err := g.doGraphQLPageShrink(ctx, g.graphQLPullRequestsQuery(), vars, &resp, graphQLPageSize(perPage, githubGraphQLPRPageSize), "pull requests"); err != nil {
		return nil, false, err
	}
	g.respectGraphQLBudget(ctx, resp.RateLimit)

	allPRs := make([]*base.PullRequest, 0, len(resp.Repository.PullRequests.Nodes))
	// PR node id -> number, for the timeline node-id pass after this page
	timelineTargets := map[string]int64{}
	// PR-comment reactions are fetched by the batched node-id pass after this
	// page is built (keyed by comment node id).
	commentReactionTargets := map[string]*base.Comment{}
	for i := range resp.Repository.PullRequests.Nodes {
		node := &resp.Repository.PullRequests.Nodes[i]

		pr, err := g.convertGraphQLPullRequest(ctx, node)
		if err != nil {
			return nil, false, err
		}
		allPRs = append(allPRs, pr)
		timelineTargets[node.ID] = node.Number
		// SECURITY: ensure the PR is safe (mirrors the REST path)
		_ = CheckAndEnsureSafePR(pr, g.baseURL, g)

		// Cache the PR's issue-comments into the shared cache (served by the
		// GetComments). Overflow -> REST for this PR's comments (the REST path
		// fetches comment reactions itself).
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

		// Cache reviews (with inline comments). If any review set or a review's
		// comment set overflows one page, fall back to REST for the whole PR's
		// reviews so nothing is dropped.
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

	// Advance only once the whole page converted: the framework retries a failed
	// page with the SAME page number, so a cursor moved on before a mid-page error
	// (the REST comment or review fallback failing) would make the retry return the
	// NEXT page and drop this one's pull requests with no error surfacing.
	g.gqlPRCursor = resp.Repository.PullRequests.PageInfo.EndCursor

	log.Info("metadata sync [%s/%s]: PRs page %d — %d pull requests, %d timeline targets",
		g.repoOwner, g.repoName, page, len(allPRs), len(timelineTargets))

	// Best-effort: reaction + timeline sweeps must not abort the PR/comment
	// import (#37).
	if err := g.attachCommentReactions(ctx, commentReactionTargets); err != nil {
		log.Error("github graphql: PR comment reactions sync failed, importing without them: %v", err)
	}
	if len(timelineTargets) > 0 {
		if err := g.attachTimelineEvents(ctx, timelineTargets); err != nil {
			log.Error("github graphql: PR timeline events sync failed, importing without them: %v", err)
		}
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
		Number:     node.Number,
		Title:      node.Title,
		PosterID:   node.Author.DatabaseID,
		PosterName: node.Author.Login,
		Content:    node.Body,
		Milestone:  milestone,
		State:      state,
		Created:    node.CreatedAt,
		Updated:    node.UpdatedAt,
		Closed:     node.ClosedAt,
		Labels:     labels,
		// A raw patch download, not an API call — construct rather than resolve it.
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

// graphQLReviewRequestsSweepQuery pages the full pending-reviewer list of a single
// pull request, addressed by its GraphQL node id (see sweepNodeConnection).
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

// reviewsWithRequestSweep converts a PR's reviews, paging the pending review
// requests by node id when the content query did not ask for all of them.
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

// convertGraphQLReviews maps a PR's GraphQL reviews (and their inline comments)
// and the given pending review requests onto base.Review, matching the REST path.
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
			ReviewerID:   r.Author.DatabaseID,
			ReviewerName: r.Author.Login,
			CommitID:     commitID,
			Content:      r.Body,
			CreatedAt:    created,
			State:        r.State, // GraphQL and REST share the review-state enum
			Comments:     convertGraphQLReviewComments(r),
		})
	}
	// Pending review requests become REQUEST_REVIEW pseudo-reviews (REST parity).
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
			PosterID:  c.Author.DatabaseID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}
	return comments
}
