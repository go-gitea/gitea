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
// GraphQL is the default path for GitHub migrations ([migrations] USE_GRAPHQL);
// the REST downloader is kept as fallback. Scope: the issues stream (issues +
// comments + reactions + labels, this file), the pull-request stream (PRs +
// comments + reviews + review threads, github_graphql_pr.go) and timeline
// events (github_graphql_timeline.go). Comment reactions are fetched by a
// batched node-id pass (attachCommentReactions); review-comment reactions are
// not fetched yet (4-deep nesting, needs its own pass — a follow-up). Comment
// pages beyond the first 100 for a single entity fall back to REST for that
// entity so nothing is silently dropped.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
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
	// The PR stream asks for fewer nodes per page: each PR carries nested
	// reviews and review comments, so 20 is the budget-safe full size.
	githubGraphQLPRPageSize = 20
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
	// maxGraphQLTransientRetries bounds retries of a response body that died
	// mid-stream (truncated JSON, connection cut while reading). The retrying
	// transport cannot retry those — the failure happens after the response
	// headers — and on a sweep that is hours long and hundreds of requests, one
	// such blip must not abort the whole run. 5xx responses and network errors
	// are NOT retried here: retryTransport (http_client.go) already owns those,
	// and retrying them again would multiply the two budgets together.
	maxGraphQLTransientRetries = 5
	// graphQLLabelPageSize / graphQLAssigneePageSize / graphQLReviewRequestPageSize
	// cap the cheap side connections carried by the content queries. Each selects
	// totalCount so an entity that exceeds its cap is swept by node id instead of
	// silently truncated.
	graphQLLabelPageSize         = 30
	graphQLAssigneePageSize      = 30
	graphQLReviewRequestPageSize = 50
)

// gqlActorFields selects an Actor's login and numeric id. The inline fragments are
// mandatory: `databaseId` is not on the Actor interface itself, and selecting it
// only `... on User` loses the id of every Bot (dependabot[bot], github-actions[bot]),
// Organization and Mannequin author — the REST path returned those, and without them
// every such author collapses onto original_author_id 0.
const gqlActorFields = "login ... on User{databaseId} ... on Bot{databaseId} ... on Organization{databaseId} ... on Mannequin{databaseId}"

// gqlReactionFields selects a reaction and its user. Reaction.user is typed User, so
// databaseId is selected directly.
const gqlReactionFields = "content user{login databaseId}"

// graphQLTransientRetryBase is the initial backoff before a truncated response is
// re-requested; doubled each attempt. A variable so tests can shrink it.
var graphQLTransientRetryBase = 2 * time.Second

// errGraphQLTransientExhausted marks a request that kept failing transiently
// through the whole retry budget — the transport's, for a 5xx or a network
// failure, or this file's, for a body that keeps dying mid-stream. Page-driven
// callers use it to shrink the page and try again: a *deterministic* 502 (one
// specific page whose response is too heavy for the backend) never succeeds at
// the same size no matter how often it is retried.
var errGraphQLTransientExhausted = errors.New("github graphql: transient retries exhausted")

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

	// Two independent budgets: rateWaits bounds how often one request waits out a
	// throttle, decodeRetries bounds retries of a body that died mid-stream.
	rateWaits, decodeRetries := 0, 0
	for {
		g.waitAndPickClient(ctx)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.graphQLEndpoint(), bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := g.getClient().Client().Do(req)
		if err != nil {
			// A network-level failure that reaches here already exhausted
			// retryTransport's budget, so retrying it a second time here would
			// only multiply the two. Report it as transient so a page-driven
			// caller shrinks the page instead (doGraphQLPageShrink).
			return fmt.Errorf("%w: request failed: %w", errGraphQLTransientExhausted, err)
		}

		if resp.StatusCode != http.StatusOK {
			detail := readGraphQLErrorBody(resp.Body)
			resp.Body.Close()
			switch {
			case resp.StatusCode >= http.StatusInternalServerError:
				// GitHub's GraphQL endpoint 502s sporadically under heavy
				// queries, and retryTransport already retried this one. A page
				// that still fails is too heavy at this size, not unlucky —
				// report transient so the caller retries it smaller.
				return fmt.Errorf("%w: status %s%s", errGraphQLTransientExhausted, resp.Status, detail)
			case isGraphQLThrottled(resp):
				// Secondary/abuse rate limit. GitHub answers those with 403 or
				// 429 and not always a Retry-After, so retryTransport may have
				// passed it straight through; wait out the window and retry in
				// place rather than aborting the whole metadata sync.
				if rateWaits < maxGraphQLRateRetries && g.waitForGraphQLRateReset(ctx, resp.Header) {
					rateWaits++
					continue
				}
				return fmt.Errorf("github graphql: rate limited, status %s%s", resp.Status, detail)
			default:
				return fmt.Errorf("github graphql: unexpected status %s%s", resp.Status, detail)
			}
		}

		var envelope struct {
			Data   json.Value     `json:"data"`
			Errors []graphQLError `json:"errors"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&envelope)
		header := resp.Header
		resp.Body.Close()
		if decodeErr != nil {
			// A 200 whose body dies mid-stream (truncated JSON, connection cut
			// while reading). The transport cannot retry this — the failure
			// happens after the response headers — so retry here.
			if retryErr := g.waitGraphQLTransient(ctx, decodeRetries, fmt.Sprintf("truncated response: %v", decodeErr)); retryErr != nil {
				return retryErr
			}
			decodeRetries++
			continue
		}

		if len(envelope.Errors) > 0 {
			// A RATE_LIMITED response means the points budget (or a secondary
			// abuse limit) is exhausted. Wait out the window and retry in place
			// rather than aborting the whole metadata sync — the REST path does
			// the same via waitAndPickClient.
			if isGraphQLRateLimited(envelope.Errors) && rateWaits < maxGraphQLRateRetries {
				if g.waitForGraphQLRateReset(ctx, header) {
					rateWaits++
					continue
				}
			}
			return fmt.Errorf("github graphql: %w", envelope.Errors[0])
		}
		return json.Unmarshal(envelope.Data, out)
	}
}

// maxGraphQLErrorBody bounds how much of a failing response body is quoted in the
// error: GitHub explains a 403/429/502 in the body, and without it the message says
// only "unexpected status" and nothing actionable.
const maxGraphQLErrorBody = 512

// readGraphQLErrorBody returns a bounded, single-line excerpt of a failing response
// body, prefixed with ": " so it appends cleanly, or "" when the body is empty.
func readGraphQLErrorBody(body io.Reader) string {
	buf, err := io.ReadAll(io.LimitReader(body, maxGraphQLErrorBody))
	if err != nil || len(buf) == 0 {
		return ""
	}
	return ": " + strings.Join(strings.Fields(string(buf)), " ")
}

// isGraphQLThrottled reports whether a non-200 response is GitHub throttling this
// client. 429 always is; a 403 only when it carries a rate-limit signal, because a
// plain 403 is a permission error that no amount of waiting fixes.
func isGraphQLThrottled(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return resp.Header.Get("Retry-After") != "" || resp.Header.Get("X-RateLimit-Remaining") == "0"
	}
	return false
}

// waitGraphQLTransient sleeps out an exponential backoff before a truncated
// response is retried. Returns nil when the caller should retry, or an error
// when the retry budget is exhausted or the context was cancelled.
func (g *GithubDownloaderV3) waitGraphQLTransient(ctx context.Context, attempt int, reason string) error {
	if attempt >= maxGraphQLTransientRetries {
		return fmt.Errorf("%w: %s", errGraphQLTransientExhausted, reason)
	}
	wait := graphQLTransientRetryBase << attempt
	log.Warn("github graphql: transient failure (%s), retrying in %s (attempt %d/%d)",
		reason, wait, attempt+1, maxGraphQLTransientRetries)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
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
// of first: down each path and rejects anything over 500,000 possible nodes, and
// separately budgets per-query compute. Reactions are kept SHALLOW: nesting them
// under comments (issues×comments×reactions) is a 3rd connection level that blows
// both limits — issues(100)×comments(100)×reactions(100) = 1,000,000 nodes is
// refused outright (MAX_NODE_LIMIT_EXCEEDED). So the content query carries only
// the issue-level reactions (one level, 100×100 = 10k, cheap); comment reactions
// are fetched afterwards by a batched node-id pass (attachCommentReactions),
// which stays two levels deep. When reactions are skipped none are requested.
//
// Issues are paged oldest-created-first: cursor pagination over a *mutable* sort
// key loses rows, so the immutable createdAt is what the cursor walks.
func (g *GithubDownloaderV3) graphQLIssuesQuery() string {
	if g.gqlIssuesQuery == "" {
		issueReactions := ""
		if !g.SkipReactions {
			issueReactions = "reactions(first:100){totalCount nodes{" + gqlReactionFields + "}}"
		}
		g.gqlIssuesQuery = fmt.Sprintf(`
query($owner:String!,$name:String!,$cursor:String,$first:Int!){
  repository(owner:$owner,name:$name){
    issues(first:$first,after:$cursor,orderBy:{field:CREATED_AT,direction:ASC}){
      pageInfo{hasNextPage endCursor}
      nodes{
        id number title body state createdAt updatedAt closedAt
        locked
        author{%[1]s}
        milestone{title}
        labels(first:%[2]d){totalCount nodes{name color description}}
        assignees(first:%[3]d){totalCount nodes{login}}
        %[4]s
        comments(first:100){
          totalCount
          nodes{
            id databaseId body createdAt updatedAt
            author{%[1]s}
          }
        }
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`, gqlActorFields, graphQLLabelPageSize, graphQLAssigneePageSize, issueReactions)
	}
	return g.gqlIssuesQuery
}

// gqlPageInfo is a connection's cursor block, shared by every paged sweep.
type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// gqlActor is a GitHub Actor (a user, bot, organization or mannequin).
type gqlActor struct {
	Login      string `json:"login"`
	DatabaseID int64  `json:"databaseId"`
}

type gqlReactionNode struct {
	Content string   `json:"content"`
	User    gqlActor `json:"user"`
}

type gqlReactionConn struct {
	// TotalCount lets the batched query detect when an entity's reactions
	// overflowed the (capped) page returned, so the remainder can be swept.
	TotalCount int               `json:"totalCount"`
	Nodes      []gqlReactionNode `json:"nodes"`
}

type gqlComment struct {
	ID         string    `json:"id"` // GraphQL node id, for the reactions node-id pass
	DatabaseID int64     `json:"databaseId"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Author     gqlActor  `json:"author"`
}

// gqlLabelConn and gqlAssigneeConn carry totalCount so an entity with more labels
// or assignees than the query's page size is swept rather than truncated.
type gqlLabelNode struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type gqlLabelConn struct {
	TotalCount int            `json:"totalCount"`
	Nodes      []gqlLabelNode `json:"nodes"`
}

type gqlAssigneeNode struct {
	Login string `json:"login"`
}

type gqlAssigneeConn struct {
	TotalCount int               `json:"totalCount"`
	Nodes      []gqlAssigneeNode `json:"nodes"`
}

type gqlIssue struct {
	ID        string     `json:"id"` // GraphQL node id, for the reactions sweep and timeline pass
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
	Labels    gqlLabelConn    `json:"labels"`
	Assignees gqlAssigneeConn `json:"assignees"`
	Reactions gqlReactionConn `json:"reactions"`
	Comments  struct {
		TotalCount int          `json:"totalCount"`
		Nodes      []gqlComment `json:"nodes"`
	} `json:"comments"`
}

type gqlIssuesResponse struct {
	Repository struct {
		Issues struct {
			PageInfo gqlPageInfo `json:"pageInfo"`
			Nodes    []gqlIssue  `json:"nodes"`
		} `json:"issues"`
	} `json:"repository"`
	RateLimit graphQLRateLimit `json:"rateLimit"`
}

// getIssuesGraphQL fetches a page of issues (oldest created first) together with
// their comments and reactions in a single request, caching the comments so
// GetComments serves them from memory instead of re-fetching. Returns the issues,
// whether this was the last page, and an error.
//
// page<=1 marks the first request of a sweep and resets the cursor, matching
// getIssuesREST.
func (g *GithubDownloaderV3) getIssuesGraphQL(ctx context.Context, page, perPage int) ([]*base.Issue, bool, error) {
	if page <= 1 {
		g.gqlIssuesCursor = ""
		g.gqlComments = map[int64][]*base.Comment{}
		log.Info("metadata sync [%s/%s]: issues — full sweep", g.repoOwner, g.repoName)
	}

	vars := map[string]any{
		"owner": g.repoOwner,
		"name":  g.repoName,
	}
	if g.gqlIssuesCursor != "" {
		vars["cursor"] = g.gqlIssuesCursor
	}

	var resp gqlIssuesResponse
	if err := g.doGraphQLPageShrink(ctx, g.graphQLIssuesQuery(), vars, &resp, graphQLPageSize(perPage, githubGraphQLPageSize), "issues"); err != nil {
		return nil, false, err
	}
	g.respectGraphQLBudget(ctx, resp.RateLimit)

	issues := make([]*base.Issue, 0, len(resp.Repository.Issues.Nodes))
	// issue node id -> number, for the timeline node-id pass after this page
	timelineTargets := map[string]int64{}
	// GraphQL-fetched comments whose reactions are pulled by the batched node-id
	// pass after this page is built (keyed by comment node id).
	commentReactionTargets := map[string]*base.Comment{}
	for i := range resp.Repository.Issues.Nodes {
		node := &resp.Repository.Issues.Nodes[i]

		labels, err := g.labelsWithSweep(ctx, node.ID, node.Labels)
		if err != nil {
			return nil, false, err
		}
		assignees, err := g.assigneesWithSweep(ctx, node.ID, node.Assignees)
		if err != nil {
			return nil, false, err
		}
		var milestone string
		if node.Milestone != nil {
			milestone = node.Milestone.Title
		}

		issueReactions, err := g.reactionsWithSweep(ctx, node.ID, node.Reactions)
		if err != nil {
			return nil, false, err
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
			Reactions:    issueReactions,
			ForeignIndex: node.Number,
		})
		timelineTargets[node.ID] = node.Number

		// Cache the comments that came back with the issue so GetComments
		// serves them for free. If the issue has more than one page of comments,
		// fall back to REST for the complete set (correctness over the fast path;
		// the REST path fetches comment reactions itself).
		if node.Comments.TotalCount > len(node.Comments.Nodes) {
			rest, err := g.getComments(ctx, &base.Issue{Number: node.Number, ForeignIndex: node.Number})
			if err != nil {
				return nil, false, err
			}
			g.gqlComments[node.Number] = rest
			continue
		}
		cached := convertGraphQLComments(node.Number, node.Comments.Nodes)
		g.gqlComments[node.Number] = cached
		for j := range cached {
			commentReactionTargets[node.Comments.Nodes[j].ID] = cached[j]
		}
	}

	// Advance only once the whole page converted: the framework retries a failed
	// page with the SAME page number, so a cursor moved on before a mid-page error
	// (a reaction sweep or the REST comment fallback failing) would make the retry
	// return the NEXT page and drop this one's issues with no error surfacing.
	g.gqlIssuesCursor = resp.Repository.Issues.PageInfo.EndCursor

	log.Info("metadata sync [%s/%s]: issues page %d — fetched %d issues, %d with timeline targets",
		g.repoOwner, g.repoName, page, len(issues), len(timelineTargets))

	// Comment reactions and timeline events are best-effort enrichments fetched
	// in separate sweeps; a failure in either must never abort the
	// issue/PR/comment import (see #37).
	if err := g.attachCommentReactions(ctx, commentReactionTargets); err != nil {
		log.Error("github graphql: comment reactions sync failed, importing without them: %v", err)
	}
	if len(timelineTargets) > 0 {
		if err := g.attachTimelineEvents(ctx, timelineTargets); err != nil {
			log.Error("github graphql: timeline events sync failed, importing without them: %v", err)
		}
	}

	return issues, !resp.Repository.Issues.PageInfo.HasNextPage, nil
}

// convertGraphQLComments maps a set of GraphQL comment nodes to base.Comment,
// preserving order (1:1 with nodes). Reactions are NOT set here — comment
// reactions are fetched separately by attachCommentReactions to keep the content
// query shallow (see graphQLIssuesQuery).
func convertGraphQLComments(issueNumber int64, nodes []gqlComment) []*base.Comment {
	comments := make([]*base.Comment, 0, len(nodes))
	for i := range nodes {
		c := &nodes[i]
		comments = append(comments, &base.Comment{
			IssueIndex: issueNumber,
			Index:      c.DatabaseID,
			PosterID:   c.Author.DatabaseID,
			PosterName: c.Author.Login,
			Content:    c.Body,
			Created:    c.CreatedAt,
			Updated:    c.UpdatedAt,
		})
	}
	return comments
}

// graphQLPageSize is the connection size for one content page: the batch size the
// framework asked for, capped at what the query shape can afford. Honouring
// perPage matters — it is derived from the uploader's max batch insert size, and a
// page larger than that overflows the DB's bind-parameter limit on insert.
func graphQLPageSize(perPage, maxSize int) int {
	if perPage <= 0 {
		return maxSize
	}
	return min(perPage, maxSize)
}

// --- single-node connection sweeps ----------------------------------------------
//
// The content queries cap every connection they carry, so an entity with more
// labels, assignees, reviewers or reactions than the cap returns short. Each of
// those is then paged in full by node id. Every such query aliases its connection
// to `conn`, so one response shape and one loop (sweepNodeConnection) serves them
// all.

// gqlNodeConnResponse is the response shape of every single-node connection sweep.
type gqlNodeConnResponse[T any] struct {
	Node struct {
		Conn struct {
			PageInfo gqlPageInfo `json:"pageInfo"`
			Nodes    []T         `json:"nodes"`
		} `json:"conn"`
	} `json:"node"`
	RateLimit graphQLRateLimit `json:"rateLimit"`
}

// sweepNodeConnection pages one node's `conn` connection to its end, starting after
// startCursor (empty for the first page), and returns every node it yields.
func sweepNodeConnection[T any](ctx context.Context, g *GithubDownloaderV3, query, nodeID, startCursor string) ([]T, error) {
	var all []T
	cursor := startCursor
	for {
		vars := map[string]any{"id": nodeID}
		if cursor != "" {
			vars["cursor"] = cursor
		}
		var resp gqlNodeConnResponse[T]
		if err := g.doGraphQL(ctx, query, vars, &resp); err != nil {
			return nil, err
		}
		g.respectGraphQLBudget(ctx, resp.RateLimit)
		all = append(all, resp.Node.Conn.Nodes...)
		if !resp.Node.Conn.PageInfo.HasNextPage {
			return all, nil
		}
		cursor = resp.Node.Conn.PageInfo.EndCursor
	}
}

// graphQLLabelsSweepQuery pages the full label list of a single Labelable (issue or
// pull request), addressed by its GraphQL node id.
const graphQLLabelsSweepQuery = `
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on Labelable{
      conn:labels(first:100,after:$cursor){
        pageInfo{hasNextPage endCursor}
        nodes{name color description}
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`

// labelsWithSweep converts an entity's labels, paging the rest by node id when the
// entity carries more of them than the content query asked for.
func (g *GithubDownloaderV3) labelsWithSweep(ctx context.Context, nodeID string, conn gqlLabelConn) ([]*base.Label, error) {
	nodes := conn.Nodes
	if conn.TotalCount > len(nodes) && nodeID != "" {
		swept, err := sweepNodeConnection[gqlLabelNode](ctx, g, graphQLLabelsSweepQuery, nodeID, "")
		if err != nil {
			return nil, err
		}
		nodes = swept
	}
	labels := make([]*base.Label, 0, len(nodes))
	for _, l := range nodes {
		labels = append(labels, &base.Label{Name: l.Name, Color: l.Color, Description: l.Description})
	}
	return labels, nil
}

// graphQLAssigneesSweepQuery pages the full assignee list of a single Assignable.
const graphQLAssigneesSweepQuery = `
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on Assignable{
      conn:assignees(first:100,after:$cursor){
        pageInfo{hasNextPage endCursor}
        nodes{login}
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`

// assigneesWithSweep converts an entity's assignee logins, paging the rest by node
// id on overflow.
func (g *GithubDownloaderV3) assigneesWithSweep(ctx context.Context, nodeID string, conn gqlAssigneeConn) ([]string, error) {
	nodes := conn.Nodes
	if conn.TotalCount > len(nodes) && nodeID != "" {
		swept, err := sweepNodeConnection[gqlAssigneeNode](ctx, g, graphQLAssigneesSweepQuery, nodeID, "")
		if err != nil {
			return nil, err
		}
		nodes = swept
	}
	assignees := make([]string, 0, len(nodes))
	for _, a := range nodes {
		assignees = append(assignees, a.Login)
	}
	return assignees, nil
}

// graphQLReactionsSweepQuery pages the full reaction list for a single node (any
// Reactable: issue, PR, or comment), addressed by its GraphQL node id.
const graphQLReactionsSweepQuery = `
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on Reactable{
      conn:reactions(first:100,after:$cursor){
        pageInfo{hasNextPage endCursor}
        nodes{` + gqlReactionFields + `}
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`

// reactionsWithSweep converts an entity's (issue/PR) reactions. The batched query
// caps the reaction page; when an entity has more reactions than the cap
// returned, sweep the remainder by node id so nothing is silently dropped.
func (g *GithubDownloaderV3) reactionsWithSweep(ctx context.Context, nodeID string, conn gqlReactionConn) ([]*base.Reaction, error) {
	if conn.TotalCount > len(conn.Nodes) && nodeID != "" {
		return g.sweepReactions(ctx, nodeID)
	}
	return convertGraphQLReactions(conn), nil
}

// sweepReactions fetches every reaction for one node over GraphQL. Used for the
// rare entity or comment whose reactions overflowed the capped batched page.
func (g *GithubDownloaderV3) sweepReactions(ctx context.Context, nodeID string) ([]*base.Reaction, error) {
	nodes, err := sweepNodeConnection[gqlReactionNode](ctx, g, graphQLReactionsSweepQuery, nodeID, "")
	if err != nil {
		return nil, err
	}
	return convertGraphQLReactions(gqlReactionConn{TotalCount: len(nodes), Nodes: nodes}), nil
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

// nodeReactionsBatchSize is how many node ids are asked for reactions per request.
// 100 ids × reactions(first:100) = 10k nodes, two levels deep — well under both
// the node cap and the per-query compute budget.
const nodeReactionsBatchSize = 100

// graphQLNodeReactionsQuery fetches reactions for a batch of nodes by id in one
// request. Two levels deep (nodes → reactions), so it avoids the compute-budget
// blowup of nesting reactions under comments in the content query. Any Reactable
// (issue, PR, comment) works; here it's used for comment ids.
const graphQLNodeReactionsQuery = `
query($ids:[ID!]!){
  nodes(ids:$ids){
    id
    ... on Reactable{reactions(first:100){totalCount nodes{` + gqlReactionFields + `}}}
  }
  rateLimit{cost remaining resetAt}
}`

// isGraphQLResourceLimited reports whether err is GitHub's
// RESOURCE_LIMITS_EXCEEDED (the per-query compute budget, distinct from
// RATE_LIMITED and the node cap).
func isGraphQLResourceLimited(err error) bool {
	var ge graphQLError
	return errors.As(err, &ge) && ge.Type == "RESOURCE_LIMITS_EXCEEDED"
}

// attachCommentReactions fetches reactions for the given comments (keyed by their
// GraphQL node id) via the batched node-id pass and sets them in place, so the
// cached comments carry reactions when they are handed over. No-op
// when reactions are skipped.
func (g *GithubDownloaderV3) attachCommentReactions(ctx context.Context, byNodeID map[string]*base.Comment) error {
	if g.SkipReactions || len(byNodeID) == 0 {
		return nil
	}
	ids := make([]string, 0, len(byNodeID))
	for id := range byNodeID {
		if id != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids) // deterministic batching
	for start := 0; start < len(ids); start += nodeReactionsBatchSize {
		end := min(start+nodeReactionsBatchSize, len(ids))
		byID, err := g.fetchNodeReactions(ctx, ids[start:end])
		if err != nil {
			return err
		}
		for id, reactions := range byID {
			if c := byNodeID[id]; c != nil {
				c.Reactions = reactions
			}
		}
	}
	return nil
}

// fetchNodeReactions returns reactions for a batch of node ids. On
// RESOURCE_LIMITS_EXCEEDED it halves the batch and retries (adaptive shrink)
// rather than aborting; a node whose reactions overflow one page falls back to
// the per-node sweep.
func (g *GithubDownloaderV3) fetchNodeReactions(ctx context.Context, ids []string) (map[string][]*base.Reaction, error) {
	if len(ids) == 0 {
		return map[string][]*base.Reaction{}, nil
	}
	var resp struct {
		Nodes []struct {
			ID        string          `json:"id"`
			Reactions gqlReactionConn `json:"reactions"`
		} `json:"nodes"`
		RateLimit graphQLRateLimit `json:"rateLimit"`
	}
	if err := g.doGraphQL(ctx, graphQLNodeReactionsQuery, map[string]any{"ids": ids}, &resp); err != nil {
		if isGraphQLResourceLimited(err) && len(ids) > 1 {
			mid := len(ids) / 2
			left, err := g.fetchNodeReactions(ctx, ids[:mid])
			if err != nil {
				return nil, err
			}
			right, err := g.fetchNodeReactions(ctx, ids[mid:])
			if err != nil {
				return nil, err
			}
			maps.Copy(left, right)
			return left, nil
		}
		return nil, err
	}
	g.respectGraphQLBudget(ctx, resp.RateLimit)

	out := make(map[string][]*base.Reaction, len(resp.Nodes))
	for _, n := range resp.Nodes {
		if n.ID == "" {
			continue // null node (unresolvable id)
		}
		if n.Reactions.TotalCount > len(n.Reactions.Nodes) {
			swept, err := g.sweepReactions(ctx, n.ID)
			if err != nil {
				return nil, err
			}
			out[n.ID] = swept
			continue
		}
		out[n.ID] = convertGraphQLReactions(n.Reactions)
	}
	return out, nil
}

// doGraphQLPageShrink runs a page query whose connection size is the $first
// variable, halving the page and re-requesting the same cursor whenever the
// transient retry budget is exhausted. A deterministic 502 — one specific page
// whose response payload is too heavy for GitHub's backend — never succeeds at
// the same size, but does at a smaller one; the cursor still advances by
// however many nodes the smaller page returned, and the next page starts back
// at full size.
func (g *GithubDownloaderV3) doGraphQLPageShrink(ctx context.Context, query string, vars map[string]any, out any, fullSize int, what string) error {
	for pageSize := fullSize; ; {
		vars["first"] = pageSize
		err := g.doGraphQL(ctx, query, vars, out)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errGraphQLTransientExhausted) || pageSize <= 1 {
			return err
		}
		pageSize = max(1, pageSize/2)
		log.Warn("github graphql: %s page keeps failing transiently — shrinking page size to %d and retrying", what, pageSize)
	}
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
