// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

// Optional GraphQL-backed fast path for the GitHub migration downloader.
//
// The REST downloader makes many small calls — worst of all a separate
// reactions request per issue and per comment — so a large repository exhausts
// the 5,000 req/hr limit long before it finishes. GitHub's GraphQL API returns
// an issue together with its comments, reactions and labels in a single request
// (100 nodes/page) and is billed by a separate points budget whose cost model is
// far more forgiving for this batched shape. This collapses dozens of REST calls
// into one.
//
// GraphQL is the default path for GitHub migrations. The REST downloader is
// kept as fallback. Scope: the issues stream (issues + their comments + reactions +
// labels). Pull-request reviews/review-threads over GraphQL are a TODO — they
// still go through the REST path. Comment/reaction pages beyond the first 100 for
// a single entity fall back to REST for that entity so nothing is silently
// dropped.

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
)

const (
	// GraphQL returns at most 100 nodes per connection page.
	githubGraphQLPageSize = 100
	// Pause when the GraphQL points budget drops to this, mirroring the REST
	// GithubLimitRateRemaining guard.
	githubGraphQLPointsFloor = 10
	// maxGraphQLRateRetries bounds how many times a single request waits out a
	// RATE_LIMITED response before giving up, so a persistently throttled sync
	// fails loudly instead of looping forever.
	maxGraphQLRateRetries = 5
	// graphQLRateResetCap bounds how long a single RATE_LIMITED wait can sleep,
	// guarding against a bogus reset header.
	graphQLRateResetCap = time.Hour
	// graphQLRateResetFallback is used when a RATE_LIMITED response carries no
	// usable reset header.
	graphQLRateResetFallback = time.Minute
)

// graphQLRequest is the POST body for a GraphQL call.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// graphQLError is one entry in a GraphQL response's top-level errors array.
type graphQLError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e graphQLError) Error() string {
	if e.Type != "" {
		return e.Type + ": " + e.Message
	}
	return e.Message
}

// graphQLRateLimit is GitHub's `rateLimit { ... }` block (the points budget,
// distinct from the REST 5,000 req/hr limit).
type graphQLRateLimit struct {
	Cost      int       `json:"cost"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

// graphQLEndpoint derives the GraphQL URL from the REST base URL. github.com uses
// a dedicated host; GitHub Enterprise serves it at {base}/api/graphql.
func (g *GithubDownloaderV3) graphQLEndpoint() string {
	if g.baseURL == "" || g.baseURL == "https://github.com" {
		return "https://api.github.com/graphql"
	}
	return strings.TrimSuffix(g.baseURL, "/") + "/api/graphql"
}

// doGraphQL executes a query and unmarshals the `data` field into out. It reuses
// the currently selected client's authenticated HTTP client (so it inherits the
// oauth2 token and the retrying transport), and returns the response's rateLimit
// block for budget accounting.
func (g *GithubDownloaderV3) doGraphQL(ctx context.Context, query string, vars map[string]any, out any) error {
	payload, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}

	for attempt := 0; ; attempt++ {
		g.waitAndPickClient(ctx)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.graphQLEndpoint(), bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := g.getClient().Client().Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("github graphql: unexpected status %s", resp.Status)
		}

		var envelope struct {
			Data   json.Value     `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&envelope)
		header := resp.Header
		resp.Body.Close()
		if decodeErr != nil {
			return decodeErr
		}

		if len(envelope.Errors) > 0 {
			// A RATE_LIMITED response means the points budget (or a secondary
			// abuse limit) is exhausted. Wait out the window and retry in place
			// rather than aborting the whole metadata sync — the REST path does
			// the same via waitAndPickClient.
			if isGraphQLRateLimited(envelope.Errors) && attempt < maxGraphQLRateRetries {
				if g.waitForGraphQLRateReset(ctx, header) {
					continue
				}
			}
			return fmt.Errorf("github graphql: %w", envelope.Errors[0])
		}
		return json.Unmarshal(envelope.Data, out)
	}
}

// isGraphQLRateLimited reports whether any of the returned errors is GitHub's
// RATE_LIMITED type (points budget or secondary abuse limit exhausted).
func isGraphQLRateLimited(errs []graphQLError) bool {
	for _, e := range errs {
		if e.Type == "RATE_LIMITED" {
			return true
		}
	}
	return false
}

// waitForGraphQLRateReset sleeps until the GitHub rate-limit window resets so a
// RATE_LIMITED response is waited out in-process. Returns false if the context is
// cancelled while waiting.
func (g *GithubDownloaderV3) waitForGraphQLRateReset(ctx context.Context, header http.Header) bool {
	wait := graphQLRateResetWait(header)
	log.Info("github graphql: RATE_LIMITED, sleeping %s until reset", wait.Round(time.Second))
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// graphQLRateResetWait derives how long to wait from GitHub's rate headers:
// Retry-After (secondary limits, in seconds) takes precedence, then
// X-RateLimit-Reset (primary budget, a unix epoch). It falls back to a short
// delay when neither is present and is capped to guard against a bogus header.
func graphQLRateResetWait(header http.Header) time.Duration {
	wait := graphQLRateResetFallback
	if ra := header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			wait = time.Duration(secs) * time.Second
		}
	} else if reset := header.Get("X-RateLimit-Reset"); reset != "" {
		if epoch, err := strconv.ParseInt(reset, 10, 64); err == nil {
			if d := time.Until(time.Unix(epoch, 0)); d > 0 {
				wait = d
			}
		}
	}
	if wait > graphQLRateResetCap {
		wait = graphQLRateResetCap
	}
	return wait
}

// respectGraphQLBudget pauses until the points budget resets when it is nearly
// exhausted, so a long backfill sleeps in-process rather than erroring.
func (g *GithubDownloaderV3) respectGraphQLBudget(ctx context.Context, rl graphQLRateLimit) {
	// Benchmark instrumentation: GraphQL is billed on a points budget (not the REST
	// requests/hr limit), so record per-query cost and the running total for the run.
	g.gqlPointsSpent += int64(rl.Cost)
	log.Trace("github graphql: query cost=%d remaining=%d cumulative=%d", rl.Cost, rl.Remaining, g.gqlPointsSpent)

	if rl.Remaining > githubGraphQLPointsFloor || rl.ResetAt.IsZero() {
		return
	}
	wait := time.Until(rl.ResetAt)
	if wait <= 0 {
		return
	}
	log.Info("github graphql: points budget low (%d), sleeping %s until reset", rl.Remaining, wait.Round(time.Second))
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// --- issues query -----------------------------------------------------------

// graphQLIssuesQuery builds the issues query. GitHub costs a query by the product
// of first: down each path and rejects anything over 500,000 possible nodes, so
// nesting reactions under comments — issues(100)×comments(100)×reactions(100) =
// 1,000,000 — is refused outright (MAX_NODE_LIMIT_EXCEEDED). Reactions are also
// exactly what a mirror skips (SkipReactions), so we never nest them under
// comments and include the cheap issue-level reactions only when reactions are
// actually wanted (a non-mirror migration). This keeps the query at ~16k nodes
// (skip) / ~26k (with reactions), far under the ceiling.
func (g *GithubDownloaderV3) graphQLIssuesQuery() string {
	issueReactions := ""
	if !g.SkipReactions {
		issueReactions = "reactions(first:100){nodes{content user{login ... on User{databaseId}}}}"
	}
	return fmt.Sprintf(`
query($owner:String!,$name:String!,$cursor:String,$since:DateTime){
  repository(owner:$owner,name:$name){
    issues(first:100,after:$cursor,orderBy:{field:UPDATED_AT,direction:ASC},filterBy:{since:$since}){
      pageInfo{hasNextPage endCursor}
      nodes{
        id number title body state createdAt updatedAt closedAt
        locked
        author{login ... on User{databaseId}}
        milestone{title}
        labels(first:30){nodes{name color description}}
        assignees(first:30){nodes{login}}
        %s
        comments(first:100){
          totalCount
          nodes{
            databaseId body createdAt updatedAt
            author{login ... on User{databaseId}}
          }
        }
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`, issueReactions)
}

// gqlActor is a GitHub Actor; databaseId is only populated for real users.
type gqlActor struct {
	Login      string `json:"login"`
	DatabaseID int64  `json:"databaseId"`
}

type gqlReactionConn struct {
	Nodes []struct {
		Content string   `json:"content"`
		User    gqlActor `json:"user"`
	} `json:"nodes"`
}

type gqlComment struct {
	DatabaseID int64           `json:"databaseId"`
	Body       string          `json:"body"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	Author     gqlActor        `json:"author"`
	Reactions  gqlReactionConn `json:"reactions"`
}

type gqlIssue struct {
	ID        string     `json:"id"` // GraphQL node id, for the timeline node-id pass
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	Locked    bool       `json:"locked"` // the real locked boolean (activeLockReason is null when locked without a reason)
	Author    gqlActor   `json:"author"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	Labels struct {
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
}

type gqlIssuesResponse struct {
	Repository struct {
		Issues struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []gqlIssue `json:"nodes"`
		} `json:"issues"`
	} `json:"repository"`
	RateLimit graphQLRateLimit `json:"rateLimit"`
}

// getNewIssuesGraphQL fetches a page of issues (updated-ascending, resumable like
// the REST path) together with their comments and reactions in a single request,
// caching the comments so the framework's separate comment phase serves them from
// memory instead of re-fetching. Returns the issues, whether this was the last
// page, and an error.
//
// page<=1 marks the first request of a sweep and resets the cursor, matching
// getIssuesSince.
func (g *GithubDownloaderV3) getNewIssuesGraphQL(ctx context.Context, page int, since time.Time) ([]*base.Issue, bool, error) {
	if page <= 1 {
		g.gqlIssuesCursor = ""
		g.gqlComments = map[int64][]*base.Comment{}
		if since.IsZero() {
			log.Info("metadata sync [%s/%s]: issues — full sweep (no watermark)", g.repoOwner, g.repoName)
		} else {
			log.Info("metadata sync [%s/%s]: issues — incremental since %s", g.repoOwner, g.repoName, since.Format(time.RFC3339))
		}
	}

	vars := map[string]any{
		"owner": g.repoOwner,
		"name":  g.repoName,
	}
	if g.gqlIssuesCursor != "" {
		vars["cursor"] = g.gqlIssuesCursor
	}
	if !since.IsZero() {
		vars["since"] = since.Format(time.RFC3339)
	}

	var resp gqlIssuesResponse
	if err := g.doGraphQL(ctx, g.graphQLIssuesQuery(), vars, &resp); err != nil {
		return nil, false, err
	}
	g.respectGraphQLBudget(ctx, resp.RateLimit)
	g.gqlIssuesCursor = resp.Repository.Issues.PageInfo.EndCursor

	issues := make([]*base.Issue, 0, len(resp.Repository.Issues.Nodes))
	// issue node id -> number, for the timeline node-id pass after this page
	timelineTargets := map[string]int64{}
	for i := range resp.Repository.Issues.Nodes {
		node := &resp.Repository.Issues.Nodes[i]

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

		issues = append(issues, &base.Issue{
			Number:       node.Number,
			Title:        node.Title,
			Content:      node.Body,
			PosterID:     node.Author.DatabaseID,
			PosterName:   node.Author.Login,
			State:        strings.ToLower(node.State),
			Milestone:    milestone,
			Created:      node.CreatedAt,
			Updated:      node.UpdatedAt,
			Closed:       node.ClosedAt,
			IsLocked:     node.Locked,
			Labels:       labels,
			Assignees:    assignees,
			Reactions:    convertGraphQLReactions(node.Reactions),
			ForeignIndex: node.Number,
		})
		timelineTargets[node.ID] = node.Number

		// Cache the comments that came back with the issue so the comment phase
		// serves them for free. If the issue has more than one page of comments,
		// fall back to REST for the complete set (correctness over the fast path).
		if node.Comments.TotalCount > len(node.Comments.Nodes) {
			rest, err := g.getComments(ctx, &base.Issue{Number: node.Number, ForeignIndex: node.Number})
			if err != nil {
				return nil, false, err
			}
			g.gqlComments[node.Number] = rest
			continue
		}
		g.gqlComments[node.Number] = convertGraphQLComments(node.Number, node.Comments.Nodes)
	}

	log.Info("metadata sync [%s/%s]: issues page %d — fetched %d issues, %d with timeline targets",
		g.repoOwner, g.repoName, page, len(issues), len(timelineTargets))

	// Timeline events are a best-effort enrichment fetched in a separate sweep;
	// a failure here must never abort the issue/PR/comment import (see #37).
	if len(timelineTargets) > 0 {
		if err := g.attachTimelineEvents(ctx, timelineTargets); err != nil {
			log.Error("github graphql: timeline events sync failed, importing without them: %v", err)
		}
	}

	return issues, !resp.Repository.Issues.PageInfo.HasNextPage, nil
}

func convertGraphQLComments(issueNumber int64, nodes []gqlComment) []*base.Comment {
	comments := make([]*base.Comment, 0, len(nodes))
	for _, c := range nodes {
		comments = append(comments, &base.Comment{
			IssueIndex: issueNumber,
			Index:      c.DatabaseID,
			PosterID:   c.Author.DatabaseID,
			PosterName: c.Author.Login,
			Content:    c.Body,
			Created:    c.CreatedAt,
			Updated:    c.UpdatedAt,
			Reactions:  convertGraphQLReactions(c.Reactions),
		})
	}
	return comments
}

// convertGraphQLReactions maps GitHub's GraphQL reaction content enum
// (THUMBS_UP, …) onto the REST/Gitea content strings (+1, …).
func convertGraphQLReactions(conn gqlReactionConn) []*base.Reaction {
	if len(conn.Nodes) == 0 {
		return nil
	}
	reactions := make([]*base.Reaction, 0, len(conn.Nodes))
	for _, r := range conn.Nodes {
		reactions = append(reactions, &base.Reaction{
			UserID:   r.User.DatabaseID,
			UserName: r.User.Login,
			Content:  graphQLReactionContent(r.Content),
		})
	}
	return reactions
}

// getCachedComments serves the comments gathered by the GraphQL issue sweep,
// paginated to match the framework's comment phase. It flattens the per-issue
// cache once (deterministically ordered by issue number) and returns slices of
// it, so the comment phase makes no additional API calls.
func (g *GithubDownloaderV3) getCachedComments(page, perPage int) ([]*base.Comment, bool, error) {
	if g.gqlCommentsFlat == nil {
		nums := make([]int64, 0, len(g.gqlComments))
		for n := range g.gqlComments {
			nums = append(nums, n)
		}
		slices.Sort(nums)
		// non-nil sentinel so an empty sweep isn't re-flattened every call
		g.gqlCommentsFlat = make([]*base.Comment, 0)
		for _, n := range nums {
			g.gqlCommentsFlat = append(g.gqlCommentsFlat, g.gqlComments[n]...)
		}
	}
	if perPage <= 0 {
		perPage = githubGraphQLPageSize
	}
	start := (page - 1) * perPage
	if start < 0 || start >= len(g.gqlCommentsFlat) {
		return nil, true, nil
	}
	end := min(start+perPage, len(g.gqlCommentsFlat))
	return g.gqlCommentsFlat[start:end], end >= len(g.gqlCommentsFlat), nil
}

func graphQLReactionContent(enum string) string {
	switch enum {
	case "THUMBS_UP":
		return "+1"
	case "THUMBS_DOWN":
		return "-1"
	case "LAUGH":
		return "laugh"
	case "HOORAY":
		return "hooray"
	case "CONFUSED":
		return "confused"
	case "HEART":
		return "heart"
	case "ROCKET":
		return "rocket"
	case "EYES":
		return "eyes"
	default:
		return strings.ToLower(enum)
	}
}
