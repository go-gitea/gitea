// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

// GraphQL fast path for pull requests — the follow-on to the issues path in
// github_graphql.go. It fetches a pull request together with its comments,
// reviews and review (inline) comments in a single batched request instead of
// the REST path's per-PR ListReviews + per-review ListReviewComments N+1.
//
// Reviews are cached so the framework's separate review phase serves them from
// memory; a PR's issue-comments join the shared comment cache so the comment
// phase serves issue and PR comments together. The patch URL is constructed (a
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
func (g *GithubDownloaderV3) graphQLPullRequestsQuery() string {
	prReactions := ""
	if !g.SkipReactions {
		prReactions = "reactions(first:100){totalCount nodes{content user{login ... on User{databaseId}}}}"
	}
	return fmt.Sprintf(`
query($owner:String!,$name:String!,$cursor:String,$first:Int!){
  repository(owner:$owner,name:$name){
    pullRequests(first:$first,after:$cursor,orderBy:{field:UPDATED_AT,direction:DESC},states:[OPEN,CLOSED,MERGED]){
      pageInfo{hasNextPage endCursor}
      nodes{
        id number title body state createdAt updatedAt closedAt mergedAt isDraft
        locked
        author{login ... on User{databaseId}}
        milestone{title}
        mergeCommit{oid}
        headRefName headRefOid baseRefName baseRefOid
        headRepository{name url owner{login}}
        baseRepository{name owner{login}}
        labels(first:30){nodes{name color description}}
        assignees(first:30){nodes{login}}
        %s
        comments(first:100){totalCount nodes{id databaseId body createdAt updatedAt author{login ... on User{databaseId}}}}
        reviews(first:50){totalCount nodes{
          databaseId state body createdAt submittedAt
          author{login ... on User{databaseId}}
          commit{oid}
          comments(first:50){totalCount nodes{databaseId body path diffHunk position commit{oid} author{login ... on User{databaseId}} createdAt updatedAt replyTo{databaseId}}}
        }}
        reviewRequests(first:50){nodes{requestedReviewer{... on User{login databaseId}}}}
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`, prReactions)
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
	HeadRefName string      `json:"headRefName"`
	HeadRefOID  string      `json:"headRefOid"`
	BaseRefName string      `json:"baseRefName"`
	BaseRefOID  string      `json:"baseRefOid"`
	HeadRepo    *gqlRepoRef `json:"headRepository"`
	BaseRepo    *gqlRepoRef `json:"baseRepository"`
	Labels      struct {
		Nodes []struct {
			Name        string `json:"name"`
			Color       string `json:"color"`
			Description string `json:"description"`
		} `json:"nodes"`
	} `json:"labels"`
	Assignees struct {
		Nodes []struct {
			Login string `json:"login"`
		} `json:"nodes"`
	} `json:"assignees"`
	Reactions gqlReactionConn `json:"reactions"`
	Comments  struct {
		TotalCount int          `json:"totalCount"`
		Nodes      []gqlComment `json:"nodes"`
	} `json:"comments"`
	Reviews struct {
		TotalCount int         `json:"totalCount"`
		Nodes      []gqlReview `json:"nodes"`
	} `json:"reviews"`
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer gqlActor `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`
}

type gqlPullRequestsResponse struct {
	Repository struct {
		PullRequests struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []gqlPullRequest `json:"nodes"`
		} `json:"pullRequests"`
	} `json:"repository"`
	RateLimit graphQLRateLimit `json:"rateLimit"`
}

// getNewPullRequestsGraphQL fetches a page of pull requests (updated-ascending,
// resumable like the REST path) together with their comments, reviews and review
// comments in one request. Comments are cached alongside the issue comments and
// reviews are cached per PR for the framework's later phases. page<=1 marks the
// first request of a sweep.
func (g *GithubDownloaderV3) getNewPullRequestsGraphQL(ctx context.Context, page int, since time.Time) ([]*base.PullRequest, bool, error) {
	if page <= 1 {
		g.gqlPRCursor = ""
		g.gqlReviews = map[int64][]*base.Review{}
		if since.IsZero() {
			log.Info("metadata sync [%s/%s]: pull requests — full sweep (no watermark)", g.repoOwner, g.repoName)
		} else {
			log.Info("metadata sync [%s/%s]: pull requests — newest-first, stopping at watermark %s", g.repoOwner, g.repoName, since.Format(time.RFC3339))
		}
	}
	if g.gqlComments == nil {
		g.gqlComments = map[int64][]*base.Comment{}
	}

	vars := map[string]any{"owner": g.repoOwner, "name": g.repoName}
	if g.gqlPRCursor != "" {
		vars["cursor"] = g.gqlPRCursor
	}

	var resp gqlPullRequestsResponse
	if err := g.doGraphQLPageShrink(ctx, g.graphQLPullRequestsQuery(), vars, &resp, githubGraphQLPRPageSize, "pull requests"); err != nil {
		return nil, false, err
	}
	g.respectGraphQLBudget(ctx, resp.RateLimit)
	g.gqlPRCursor = resp.Repository.PullRequests.PageInfo.EndCursor

	allPRs := make([]*base.PullRequest, 0, len(resp.Repository.PullRequests.Nodes))
	// PR node id -> number, for the timeline node-id pass after this page
	timelineTargets := map[string]int64{}
	// PR-comment reactions are fetched by the batched node-id pass after this
	// page is built (keyed by comment node id).
	commentReactionTargets := map[string]*base.Comment{}
	hitWatermark := false
	for i := range resp.Repository.PullRequests.Nodes {
		node := &resp.Repository.PullRequests.Nodes[i]
		// Ordered DESC by updated_at: once we hit a PR older than the watermark,
		// everything remaining is also older — stop processing this page and signal
		// the caller to stop paging (no more new PRs to find).
		if !since.IsZero() && node.UpdatedAt.Before(since) {
			hitWatermark = true
			break
		}

		pr, err := g.convertGraphQLPullRequest(ctx, node)
		if err != nil {
			return nil, false, err
		}
		allPRs = append(allPRs, pr)
		timelineTargets[node.ID] = node.Number
		// SECURITY: ensure the PR is safe (mirrors the REST path)
		_ = CheckAndEnsureSafePR(pr, g.baseURL, g)

		// Cache the PR's issue-comments into the shared cache (served by the
		// comment phase). Overflow -> REST for this PR's comments (the REST path
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
			g.gqlReviews[node.Number] = convertGraphQLReviews(node)
		}
	}

	log.Info("metadata sync [%s/%s]: PRs page %d — %d new, %d timeline targets, watermark_reached=%v",
		g.repoOwner, g.repoName, page, len(allPRs), len(timelineTargets), hitWatermark)

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

	// Stop paging when we've hit the watermark (no more new PRs) OR reached the last page.
	done := hitWatermark || !resp.Repository.PullRequests.PageInfo.HasNextPage
	return allPRs, done, nil
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
	labels := make([]*base.Label, 0, len(node.Labels.Nodes))
	for _, l := range node.Labels.Nodes {
		labels = append(labels, &base.Label{Name: l.Name, Color: l.Color, Description: l.Description})
	}
	assignees := make([]string, 0, len(node.Assignees.Nodes))
	for _, a := range node.Assignees.Nodes {
		assignees = append(assignees, a.Login)
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

// convertGraphQLReviews maps a PR's GraphQL reviews (and their inline comments)
// and its pending review requests onto base.Review, matching the REST path.
func convertGraphQLReviews(node *gqlPullRequest) []*base.Review {
	reviews := make([]*base.Review, 0, len(node.Reviews.Nodes)+len(node.ReviewRequests.Nodes))
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
	for _, rr := range node.ReviewRequests.Nodes {
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
