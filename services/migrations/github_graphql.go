// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

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

	"github.com/google/go-github/v92/github"
)

const (
	githubGraphQLPageSize    = 100
	githubGraphQLPRPageSize  = 20 // each PR carries nested reviews and review comments
	githubGraphQLPointsFloor = 10 // mirrors the REST GithubLimitRateRemaining guard
	maxGraphQLRateRetries    = 5
	graphQLRateResetCap      = time.Hour // guards against a bogus reset header
	// only for bodies dying mid-stream; 5xx and network errors are retryTransport's, retrying them here would multiply both budgets
	maxGraphQLTransientRetries = 5
	// overflowing entities are swept by node id via totalCount
	graphQLLabelPageSize         = 30
	graphQLAssigneePageSize      = 30
	graphQLReviewRequestPageSize = 50
)

// databaseId is not on the Actor interface; without every fragment bot/org/mannequin authors collapse onto id 0
const gqlActorFields = "login ... on User{databaseId} ... on Bot{databaseId} ... on Organization{databaseId} ... on Mannequin{databaseId}"

const gqlReactionFields = "content user{login databaseId}"

var graphQLTransientRetryBase = 2 * time.Second

var graphQLRateResetFallback = time.Minute // GitHub asks for at least a minute before retrying a secondary rate limit

// page-driven callers shrink the page on this: a deterministic 502 for a too-heavy page never succeeds at the same size
var errGraphQLTransientExhausted = errors.New("github graphql: transient retries exhausted")

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

func (e graphQLError) Error() string {
	if e.Type != "" {
		return e.Type + ": " + e.Message
	}
	return e.Message
}

type graphQLRateLimit struct {
	Cost      int       `json:"cost"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

func (g *GithubDownloaderV3) graphQLEndpoint() string {
	if g.baseURL == "" || g.baseURL == "https://github.com" {
		return "https://api.github.com/graphql"
	}
	return strings.TrimSuffix(g.baseURL, "/") + "/api/graphql"
}

// doGraphQL executes a query and unmarshals the `data` field into out.
func (g *GithubDownloaderV3) doGraphQL(ctx context.Context, query string, vars map[string]any, out any) error {
	payload, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}

	rateWaits, decodeRetries := 0, 0
	for {
		if err := g.pickGraphQLClient(ctx); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.graphQLEndpoint(), bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := g.getClient().Client().Do(req)
		if err != nil {
			// retryTransport already spent its budget; let the caller shrink the page
			if ctx.Err() != nil || !isTransientTransportError(err) {
				return err
			}
			return fmt.Errorf("%w: request failed: %w", errGraphQLTransientExhausted, err)
		}
		g.recordGraphQLRate(resp.Header)

		if resp.StatusCode != http.StatusOK {
			detail := readGraphQLErrorBody(resp.Body)
			resp.Body.Close()
			switch {
			case resp.StatusCode >= http.StatusInternalServerError:
				return fmt.Errorf("%w: status %s%s", errGraphQLTransientExhausted, resp.Status, detail)
			case isGraphQLThrottled(resp, detail):
				// without Retry-After retryTransport passed it through
				if rateWaits < maxGraphQLRateRetries {
					rateWaits++
					g.restGraphQLClient(resp.Header)
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
			// the body died after the headers, which the transport cannot retry
			if retryErr := g.waitGraphQLTransient(ctx, decodeRetries, fmt.Sprintf("truncated response: %v", decodeErr)); retryErr != nil {
				return retryErr
			}
			decodeRetries++
			continue
		}

		if len(envelope.Errors) > 0 && !onlyUnresolvedNodes(envelope.Errors) {
			if isGraphQLRateLimited(envelope.Errors) && rateWaits < maxGraphQLRateRetries {
				rateWaits++
				g.restGraphQLClient(header)
				continue
			}
			return fmt.Errorf("github graphql: %w", envelope.Errors[0])
		}
		return json.Unmarshal(envelope.Data, out)
	}
}

// GitHub explains a 403/429/502 only in the body
const maxGraphQLErrorBody = 512

// readGraphQLErrorBody returns a bounded single-line excerpt prefixed with ": ", or "" when empty.
func readGraphQLErrorBody(body io.Reader) string {
	buf, err := io.ReadAll(io.LimitReader(body, maxGraphQLErrorBody))
	if err != nil || len(buf) == 0 {
		return ""
	}
	return ": " + strings.Join(strings.Fields(string(buf)), " ")
}

// isGraphQLThrottled: a plain 403 is a permission error, and a secondary limit may carry neither Retry-After nor a spent quota, only its body text.
func isGraphQLThrottled(resp *http.Response, body string) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusForbidden:
		return resp.Header.Get("Retry-After") != "" ||
			resp.Header.Get(github.HeaderRateRemaining) == "0" ||
			strings.Contains(strings.ToLower(body), "secondary rate limit")
	}
	return false
}

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

// onlyUnresolvedNodes reports whether every error is a nodes(ids:) entry that no
// longer resolves (deleted since the content page); its data entry is null and skipped.
func onlyUnresolvedNodes(errs []graphQLError) bool {
	return !slices.ContainsFunc(errs, func(e graphQLError) bool {
		return e.Type != "NOT_FOUND" || len(e.Path) == 0 || e.Path[0] != "nodes"
	})
}

func isGraphQLRateLimited(errs []graphQLError) bool {
	return slices.ContainsFunc(errs, func(e graphQLError) bool { return e.Type == "RATE_LIMITED" })
}

// pickGraphQLClient selects the client with the most GraphQL points left, since GraphQL is metered apart from the REST
// quota. A client with an unknown or reset budget goes first so its budget gets learned.
func (g *GithubDownloaderV3) pickGraphQLClient(ctx context.Context) error {
	for {
		best, soonest := -1, -1
		for i, r := range g.gqlRates {
			if r == nil || !time.Now().Before(r.Reset.Time) {
				g.curClientIdx = i
				return nil
			}
			if r.Remaining > githubGraphQLPointsFloor && (best < 0 || r.Remaining > g.gqlRates[best].Remaining) {
				best = i
			}
			if soonest < 0 || r.Reset.Before(g.gqlRates[soonest].Reset.Time) {
				soonest = i
			}
		}
		if best >= 0 {
			g.curClientIdx = best
			return nil
		}
		wait := min(time.Until(g.gqlRates[soonest].Reset.Time), graphQLRateResetCap)
		log.Info("github graphql: every token's points budget is spent, sleeping %s until the earliest reset", wait.Round(time.Second))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (g *GithubDownloaderV3) recordGraphQLRate(header http.Header) {
	remaining, err := strconv.Atoi(header.Get(github.HeaderRateRemaining))
	if err != nil {
		return
	}
	rate := &github.Rate{Remaining: remaining}
	if epoch, err := strconv.ParseInt(header.Get(github.HeaderRateReset), 10, 64); err == nil {
		rate.Reset = github.Timestamp{Time: time.Unix(epoch, 0)}
	}
	g.gqlRates[g.curClientIdx] = rate
}

func (g *GithubDownloaderV3) restGraphQLClient(header http.Header) {
	wait := graphQLRateResetWait(header)
	log.Info("github graphql: token %d throttled, resting it for %s", g.curClientIdx, wait.Round(time.Second))
	g.gqlRates[g.curClientIdx] = &github.Rate{Reset: github.Timestamp{Time: time.Now().Add(wait)}}
}

// with quota left the throttle is a secondary limit, whose X-RateLimit-Reset belongs to the primary window
func isPrimaryBudgetSpent(header http.Header) bool {
	remaining := header.Get(github.HeaderRateRemaining)
	return remaining == "" || remaining == "0"
}

func graphQLRateResetWait(header http.Header) time.Duration {
	wait := graphQLRateResetFallback
	if ra := header.Get("Retry-After"); ra != "" {
		if d := parseRetryAfter(ra); d > 0 {
			wait = d
		}
	} else if reset := header.Get(github.HeaderRateReset); reset != "" && isPrimaryBudgetSpent(header) {
		if epoch, err := strconv.ParseInt(reset, 10, 64); err == nil {
			if d := time.Until(time.Unix(epoch, 0)); d > 0 {
				wait = d
			}
		}
	}
	return min(wait, graphQLRateResetCap)
}

func (g *GithubDownloaderV3) recordGraphQLBudget(rl graphQLRateLimit) {
	log.Trace("github graphql: query cost=%d remaining=%d", rl.Cost, rl.Remaining)

	if rl.ResetAt.IsZero() {
		return
	}
	g.gqlRates[g.curClientIdx] = &github.Rate{Remaining: rl.Remaining, Reset: github.Timestamp{Time: rl.ResetAt}}
}

// graphQLIssuesQuery keeps comment reactions out: issues×comments×reactions exceeds GitHub's 500,000-node limit,
// so attachCommentReactions fetches them separately. Paged by createdAt since a cursor over a mutable sort key loses rows.
func (g *GithubDownloaderV3) graphQLIssuesQuery() string {
	if g.gqlIssuesQuery == "" {
		issueReactions := ""
		if !g.SkipReactions {
			issueReactions = "reactions(first:100){totalCount nodes{" + gqlReactionFields + "}}"
		}
		comments := ""
		if !g.gqlSkipComments {
			comments = "comments(first:100){totalCount nodes{id databaseId body createdAt updatedAt author{" + gqlActorFields + "}}}"
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
        %[5]s
      }
    }
  }
  rateLimit{cost remaining resetAt}
}`, gqlActorFields, graphQLLabelPageSize, graphQLAssigneePageSize, issueReactions, comments)
	}
	return g.gqlIssuesQuery
}

type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type gqlActor struct {
	Login      string `json:"login"`
	DatabaseID int64  `json:"databaseId"`
}

// orGhost maps a deleted account, which GraphQL returns as null, to GitHub's ghost
// user like the REST API does.
func (a gqlActor) orGhost() gqlActor {
	if a.Login == "" {
		return gqlActor{Login: "ghost", DatabaseID: 10137}
	}
	return a
}

type gqlReactionNode struct {
	Content string   `json:"content"`
	User    gqlActor `json:"user"`
}

type gqlReactionConn struct {
	TotalCount int               `json:"totalCount"`
	Nodes      []gqlReactionNode `json:"nodes"`
}

type gqlComment struct {
	ID         string    `json:"id"`
	DatabaseID int64     `json:"databaseId"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Author     gqlActor  `json:"author"`
}

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
	ID        string     `json:"id"`
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	Locked    bool       `json:"locked"` // activeLockReason is null when locked without a reason
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

// getIssuesGraphQL fetches a page of issues with their comments, caching the comments for GetComments.
func (g *GithubDownloaderV3) getIssuesGraphQL(ctx context.Context, page, perPage int) ([]*base.Issue, bool, error) {
	if page <= 1 {
		g.gqlIssuesCursor = ""
		log.Info("metadata sync [%s/%s]: issues — full sweep", g.repoOwner, g.repoName)
	}

	g.gqlComments = map[int64][]*base.Comment{}

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
	g.recordGraphQLBudget(resp.RateLimit)

	issues := make([]*base.Issue, 0, len(resp.Repository.Issues.Nodes))
	timelineTargets := map[string]int64{}
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
			PosterID:     node.Author.orGhost().DatabaseID,
			PosterName:   node.Author.orGhost().Login,
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
		if g.gqlSkipComments {
			continue
		}
		timelineTargets[node.ID] = node.Number

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

	// only after the page converted: a failed page is retried with the same page number
	g.gqlIssuesCursor = resp.Repository.Issues.PageInfo.EndCursor

	log.Info("metadata sync [%s/%s]: issues page %d — fetched %d issues, %d with timeline targets",
		g.repoOwner, g.repoName, page, len(issues), len(timelineTargets))

	// best-effort enrichments that must not abort the import
	if err := g.attachCommentReactions(ctx, commentReactionTargets); err != nil {
		if ctx.Err() != nil {
			return nil, false, err
		}
		log.Error("github graphql: comment reactions sync failed, importing without them: %v", err)
	}
	if err := g.attachTimelineEvents(ctx, timelineTargets); err != nil {
		if ctx.Err() != nil {
			return nil, false, err
		}
		log.Error("github graphql: timeline events sync failed, importing without them: %v", err)
	}

	return issues, !resp.Repository.Issues.PageInfo.HasNextPage, nil
}

// convertGraphQLComments preserves order 1:1 with nodes; reactions come from attachCommentReactions.
func convertGraphQLComments(issueNumber int64, nodes []gqlComment) []*base.Comment {
	comments := make([]*base.Comment, 0, len(nodes))
	for i := range nodes {
		c := &nodes[i]
		comments = append(comments, &base.Comment{
			IssueIndex: issueNumber,
			Index:      c.DatabaseID,
			PosterID:   c.Author.orGhost().DatabaseID,
			PosterName: c.Author.orGhost().Login,
			Content:    c.Body,
			Created:    c.CreatedAt,
			Updated:    c.UpdatedAt,
		})
	}
	return comments
}

// perPage must be honoured: a larger page overflows the DB's bind-parameter limit on insert
func graphQLPageSize(perPage, maxSize int) int {
	if perPage <= 0 {
		return maxSize
	}
	return min(perPage, maxSize)
}

// gqlNodeConnResponse is shared by every single-node sweep query, which aliases its connection to `conn`.
type gqlNodeConnResponse[T any] struct {
	Node struct {
		Conn struct {
			PageInfo gqlPageInfo `json:"pageInfo"`
			Nodes    []T         `json:"nodes"`
		} `json:"conn"`
	} `json:"node"`
	RateLimit graphQLRateLimit `json:"rateLimit"`
}

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
		g.recordGraphQLBudget(resp.RateLimit)
		all = append(all, resp.Node.Conn.Nodes...)
		if !resp.Node.Conn.PageInfo.HasNextPage {
			return all, nil
		}
		cursor = resp.Node.Conn.PageInfo.EndCursor
	}
}

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

func (g *GithubDownloaderV3) reactionsWithSweep(ctx context.Context, nodeID string, conn gqlReactionConn) ([]*base.Reaction, error) {
	if conn.TotalCount > len(conn.Nodes) && nodeID != "" {
		return g.sweepReactions(ctx, nodeID)
	}
	return convertGraphQLReactions(conn), nil
}

func (g *GithubDownloaderV3) sweepReactions(ctx context.Context, nodeID string) ([]*base.Reaction, error) {
	nodes, err := sweepNodeConnection[gqlReactionNode](ctx, g, graphQLReactionsSweepQuery, nodeID, "")
	if err != nil {
		return nil, err
	}
	return convertGraphQLReactions(gqlReactionConn{TotalCount: len(nodes), Nodes: nodes}), nil
}

func convertGraphQLReactions(conn gqlReactionConn) []*base.Reaction {
	if len(conn.Nodes) == 0 {
		return nil
	}
	reactions := make([]*base.Reaction, 0, len(conn.Nodes))
	for _, r := range conn.Nodes {
		reactions = append(reactions, &base.Reaction{
			UserID:   r.User.orGhost().DatabaseID,
			UserName: r.User.orGhost().Login,
			Content:  graphQLReactionContent(r.Content),
		})
	}
	return reactions
}

const nodeReactionsBatchSize = 100

const graphQLNodeReactionsQuery = `
query($ids:[ID!]!){
  nodes(ids:$ids){
    id
    ... on Reactable{reactions(first:100){totalCount nodes{` + gqlReactionFields + `}}}
  }
  rateLimit{cost remaining resetAt}
}`

// isGraphQLResourceLimited reports GitHub's per-query compute budget refusing the query.
func isGraphQLResourceLimited(err error) bool {
	var ge graphQLError
	return errors.As(err, &ge) && ge.Type == "RESOURCE_LIMITS_EXCEEDED"
}

func (g *GithubDownloaderV3) attachCommentReactions(ctx context.Context, byNodeID map[string]*base.Comment) error {
	if g.SkipReactions || len(byNodeID) == 0 {
		return nil
	}
	for batch := range slices.Chunk(sortedNodeIDs(byNodeID), nodeReactionsBatchSize) {
		byID, err := g.fetchNodeReactions(ctx, batch)
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
	g.recordGraphQLBudget(resp.RateLimit)

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

// doGraphQLPageShrink halves $first and re-requests the same cursor while the page is too heavy.
func (g *GithubDownloaderV3) doGraphQLPageShrink(ctx context.Context, query string, vars map[string]any, out any, fullSize int, what string) error {
	for pageSize := fullSize; ; {
		vars["first"] = pageSize
		err := g.doGraphQL(ctx, query, vars, out)
		if err == nil {
			return nil
		}
		if !(errors.Is(err, errGraphQLTransientExhausted) || isGraphQLResourceLimited(err)) || pageSize <= 1 {
			return err
		}
		pageSize = max(1, pageSize/2)
		log.Warn("github graphql: %s page too heavy (%v), shrinking page size to %d and retrying", what, err, pageSize)
	}
}

func graphQLReactionContent(enum string) string {
	switch enum {
	case "THUMBS_UP":
		return "+1"
	case "THUMBS_DOWN":
		return "-1"
	default:
		return strings.ToLower(enum)
	}
}

// sortedNodeIDs sorts for deterministic batching
func sortedNodeIDs[V any](m map[string]V) []string {
	return slices.DeleteFunc(slices.Sorted(maps.Keys(m)), func(id string) bool { return id == "" })
}
