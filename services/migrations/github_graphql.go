// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"

	"github.com/google/go-github/v92/github"
)

const (
	githubGraphQLPageSize        = 100
	githubGraphQLPRPageSize      = 20
	githubGraphQLPointsFloor     = 10
	maxGraphQLRateRetries        = 5
	maxGraphQLTransientRetries   = 5
	graphQLRateResetCap          = time.Hour
	graphQLNodeBatchSize         = 100
	graphQLLabelPageSize         = 30
	graphQLAssigneePageSize      = 30
	graphQLReviewRequestPageSize = 50
)

const (
	gqlPageInfoFields = "pageInfo{hasNextPage endCursor}"
	// databaseId is not on the Actor interface, every implementation needs its own fragment
	gqlActorFields = "__typename login ... on User{databaseId} ... on Bot{databaseId} ... on Organization{databaseId} ... on Mannequin{databaseId}"
	// teams in the RequestedReviewer union have no login
	gqlReviewerFields = "__typename ... on User{login databaseId} ... on Bot{login databaseId} ... on Mannequin{login databaseId}"
	gqlReactionFields = "content user{login databaseId}"
)

var graphQLTransientRetryBase = 2 * time.Second

var graphQLRateResetFallback = time.Minute // GitHub requires a minute between secondary limit retries.

// a deterministic 502 for a too-heavy page never succeeds at the same size, so callers shrink it
var errGraphQLTransientExhausted = errors.New("github graphql: transient retries exhausted")

var _ base.Downloader = &GithubGraphQLDownloader{}

// GithubGraphQLDownloader sweeps issues and pull requests over GraphQL, the embedded REST downloader serves the rest and takes over on failure
type GithubGraphQLDownloader struct {
	*GithubDownloaderV3
	gqlRates     []*github.Rate // GraphQL has a separate rate budget
	skipComments bool
	issuesCursor string
	prCursor     string
	restFallback bool
	// REST restarts from its first page after a fallback, the migration skips what it already imported
	restIssuePageOffset int
	restPRPageOffset    int
}

func newGithubGraphQLDownloader(rest *GithubDownloaderV3, skipComments bool) *GithubGraphQLDownloader {
	return &GithubGraphQLDownloader{GithubDownloaderV3: rest, gqlRates: make([]*github.Rate, len(rest.clients)), skipComments: skipComments}
}

func (g *GithubGraphQLDownloader) GetIssues(ctx context.Context, page, perPage int) ([]*base.Issue, bool, error) {
	if !g.restFallback {
		issues, isEnd, err := g.getIssuesGraphQL(ctx, page, perPage)
		if !g.fallBackToREST(ctx, err) {
			return issues, isEnd, err
		}
		g.restIssuePageOffset = page - 1
	}
	return g.GithubDownloaderV3.GetIssues(ctx, page-g.restIssuePageOffset, perPage)
}

func (g *GithubGraphQLDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	if !g.restFallback {
		prs, isEnd, err := g.getPullRequestsGraphQL(ctx, page, perPage)
		if !g.fallBackToREST(ctx, err) {
			return prs, isEnd, err
		}
		g.restPRPageOffset = page - 1
	}
	return g.GithubDownloaderV3.GetPullRequests(ctx, page-g.restPRPageOffset, perPage)
}

// fallBackToREST switches to REST for good when GraphQL fails, e.g. on a server or token without GraphQL support
func (g *GithubGraphQLDownloader) fallBackToREST(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil {
		return false
	}
	log.Warn("GitHub GraphQL failed, falling back to REST: %v", err)
	g.restFallback = true
	return true
}

// SupportGetRepoComments is false so comments are served per issue from what each sweep page prefetched
func (g *GithubGraphQLDownloader) SupportGetRepoComments() bool {
	return false
}

func (g *GithubGraphQLDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	if prefetched, ok := commentable.GetContext().(*gqlPrefetched); ok {
		return prefetched.comments, false, nil
	}
	return g.GithubDownloaderV3.GetComments(ctx, commentable)
}

func (g *GithubGraphQLDownloader) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	if prefetched, ok := reviewable.GetContext().(*gqlPrefetched); ok {
		return prefetched.reviews, nil
	}
	return g.GithubDownloaderV3.GetReviews(ctx, reviewable)
}

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

func (g *GithubGraphQLDownloader) graphQLEndpoint() string {
	restURL := g.clients[0].BaseURL()
	if host, ok := strings.CutSuffix(restURL, "/api/v3/"); ok {
		return host + "/api/graphql"
	}
	return restURL + "graphql"
}

func (g *GithubGraphQLDownloader) doGraphQL(ctx context.Context, query string, vars map[string]any, out any) error {
	payload, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}

	rateWaits, transientRetries := 0, 0
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
			if ctx.Err() != nil || !isTransientTransportError(err) {
				return err
			}
			if err := g.waitGraphQLTransient(ctx, &transientRetries, fmt.Sprintf("request failed: %v", err)); err != nil {
				return err
			}
			continue
		}
		g.recordGraphQLRate(resp.Header)

		if resp.StatusCode != http.StatusOK {
			detail := readGraphQLErrorBody(resp.Body)
			resp.Body.Close()
			switch {
			case resp.StatusCode >= http.StatusInternalServerError:
				if err := g.waitGraphQLTransient(ctx, &transientRetries, "status "+resp.Status+detail); err != nil {
					return err
				}
				continue
			case isGraphQLThrottled(resp, detail):
				if rateWaits < maxGraphQLRateRetries*len(g.clients) {
					rateWaits++
					g.restGraphQLClient(resp.Header)
					continue
				}
				return fmt.Errorf("doGraphQL: rate limited, status %s%s", resp.Status, detail)
			default:
				return fmt.Errorf("doGraphQL: unexpected status %s%s", resp.Status, detail)
			}
		}

		envelope := struct {
			Data   any            `json:"data"`
			Errors []graphQLError `json:"errors"`
		}{Data: out}
		decodeErr := json.NewDecoder(resp.Body).Decode(&envelope)
		resp.Body.Close()
		if decodeErr != nil {
			if err := g.waitGraphQLTransient(ctx, &transientRetries, fmt.Sprintf("truncated response: %v", decodeErr)); err != nil {
				return err
			}
			continue
		}

		if !onlyUnresolvedNodes(envelope.Errors) {
			if isGraphQLRateLimited(envelope.Errors) && rateWaits < maxGraphQLRateRetries*len(g.clients) {
				rateWaits++
				g.restGraphQLClient(resp.Header)
				continue
			}
			return fmt.Errorf("doGraphQL: %w", envelope.Errors[0])
		}
		return nil
	}
}

const maxGraphQLErrorBody = 512

func readGraphQLErrorBody(body io.Reader) string {
	buf, err := io.ReadAll(io.LimitReader(body, maxGraphQLErrorBody))
	if err != nil || len(buf) == 0 {
		return ""
	}
	return ": " + strings.Join(strings.Fields(string(buf)), " ")
}

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

func isTransientTransportError(err error) bool {
	if errors.Is(err, policy.ErrDenied) {
		return false
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return false
	}
	if dnsErr, ok := errors.AsType[*net.DNSError](err); ok {
		return !dnsErr.IsNotFound
	}
	return true
}

func (g *GithubGraphQLDownloader) waitGraphQLTransient(ctx context.Context, attempt *int, reason string) error {
	if *attempt >= maxGraphQLTransientRetries {
		return fmt.Errorf("%w: %s", errGraphQLTransientExhausted, reason)
	}
	wait := graphQLTransientRetryBase << *attempt
	*attempt++
	log.Warn("github graphql: transient failure (%s), retrying in %s (attempt %d/%d)",
		reason, wait, *attempt, maxGraphQLTransientRetries)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func onlyUnresolvedNodes(errs []graphQLError) bool {
	return !slices.ContainsFunc(errs, func(e graphQLError) bool {
		return e.Type != "NOT_FOUND" || len(e.Path) == 0 || e.Path[0] != "nodes"
	})
}

func isGraphQLRateLimited(errs []graphQLError) bool {
	return slices.ContainsFunc(errs, func(e graphQLError) bool { return e.Type == "RATE_LIMITED" })
}

func isGraphQLTooHeavy(err error) bool {
	var ge graphQLError
	return errors.Is(err, errGraphQLTransientExhausted) || errors.As(err, &ge) && ge.Type == "RESOURCE_LIMITS_EXCEEDED"
}

func (g *GithubGraphQLDownloader) pickGraphQLClient(ctx context.Context) error {
	for {
		best, soonest := -1, -1
		for i, rate := range g.gqlRates {
			if rate == nil || !time.Now().Before(rate.Reset.Time) {
				g.curClientIdx = i
				return nil
			}
			if rate.Remaining > githubGraphQLPointsFloor && (best < 0 || rate.Remaining > g.gqlRates[best].Remaining) {
				best = i
			}
			if soonest < 0 || rate.Reset.Before(g.gqlRates[soonest].Reset.Time) {
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
			g.gqlRates[soonest] = nil // the reset may lie beyond the capped sleep
		}
	}
}

func (g *GithubGraphQLDownloader) recordGraphQLRate(header http.Header) {
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

func (g *GithubGraphQLDownloader) restGraphQLClient(header http.Header) {
	wait := graphQLRateResetWait(header)
	log.Info("github graphql: token %d throttled, resting it for %s", g.curClientIdx, wait.Round(time.Second))
	g.gqlRates[g.curClientIdx] = &github.Rate{Reset: github.Timestamp{Time: time.Now().Add(wait)}}
}

func isPrimaryBudgetSpent(header http.Header) bool {
	remaining := header.Get(github.HeaderRateRemaining)
	return remaining == "" || remaining == "0"
}

func graphQLRateResetWait(header http.Header) time.Duration {
	wait := graphQLRateResetFallback
	if ra := header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil && secs > 0 {
			wait = time.Duration(secs) * time.Second
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

// graphQLContentQuery orders by creation since a mutable sort key skips rows, nested comment reactions exceed GitHub's node limit
func (g *GithubGraphQLDownloader) graphQLContentQuery(connection, prFields, prDiscussion string) string {
	fields := prFields
	if !g.SkipReactions {
		fields += " reactions(first:100){" + gqlPageInfoFields + " nodes{" + gqlReactionFields + "}}"
	}
	if !g.skipComments {
		fields += " comments(first:100){" + gqlPageInfoFields + " nodes{id databaseId body createdAt updatedAt author{" + gqlActorFields + "}}}" + prDiscussion
	}
	return fmt.Sprintf(`
query($owner:String!,$name:String!,$cursor:String,$first:Int!){
  repository(owner:$owner,name:$name){
    %[1]s(first:$first,after:$cursor,orderBy:{field:CREATED_AT,direction:ASC}){
      %[2]s
      nodes{
        id number title body state createdAt updatedAt closedAt locked
        author{%[3]s}
        milestone{title}
        labels(first:%[4]d){%[2]s nodes{name color description}}
        assignees(first:%[5]d){%[2]s nodes{login}}
        %[6]s
      }
    }
  }
}`, connection, gqlPageInfoFields, gqlActorFields, graphQLLabelPageSize, graphQLAssigneePageSize, fields)
}

func (g *GithubGraphQLDownloader) graphQLIssuesQuery() string {
	return g.graphQLContentQuery("issues", "", "")
}

func nodeConnQuery(onType, connection, nodeFields string) string {
	return `query($id:ID!,$cursor:String){node(id:$id){... on ` + onType + `{conn:` + connection +
		`(first:100,after:$cursor){` + gqlPageInfoFields + ` nodes{` + nodeFields + `}}}}}`
}

var (
	graphQLLabelsSweepQuery    = nodeConnQuery("Labelable", "labels", "name color description")
	graphQLAssigneesSweepQuery = nodeConnQuery("Assignable", "assignees", "login")
	graphQLReactionsSweepQuery = nodeConnQuery("Reactable", "reactions", gqlReactionFields)
)

const graphQLNodeReactionsQuery = `query($ids:[ID!]!){nodes(ids:$ids){id ... on Reactable{reactions(first:100){` +
	gqlPageInfoFields + ` nodes{` + gqlReactionFields + `}}}}}`

type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type gqlConn[T any] struct {
	PageInfo gqlPageInfo `json:"pageInfo"`
	Nodes    []T         `json:"nodes"`
}

type gqlActor struct {
	Typename   string `json:"__typename"`
	Login      string `json:"login"`
	DatabaseID int64  `json:"databaseId"`
}

func (a *gqlActor) UnmarshalJSON(data []byte) error {
	type plain gqlActor
	if err := json.Unmarshal(data, (*plain)(a)); err != nil {
		return err
	}
	switch {
	case a.Typename == "" && a.Login == "":
		a.Login = "ghost" // Match REST's ghost login with an unset ID for GHES.
	case a.Typename == "Bot":
		a.Login += "[bot]"
	}
	return nil
}

type gqlCommit struct {
	OID string `json:"oid"`
}

type gqlReaction struct {
	Content string   `json:"content"`
	User    gqlActor `json:"user"`
}

type gqlComment struct {
	ID         string    `json:"id"`
	DatabaseID int64     `json:"databaseId"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Author     gqlActor  `json:"author"`
}

type gqlLabel struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
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
	Locked    bool       `json:"locked"`
	Author    gqlActor   `json:"author"`
	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`
	Labels    gqlConn[gqlLabel]    `json:"labels"`
	Assignees gqlConn[gqlActor]    `json:"assignees"`
	Reactions gqlConn[gqlReaction] `json:"reactions"`
	Comments  gqlConn[gqlComment]  `json:"comments"`
}

type gqlIssuesResponse struct {
	Repository struct {
		Issues gqlConn[gqlIssue] `json:"issues"`
	} `json:"repository"`
}

type gqlPrefetched struct {
	index      int64
	comments   []*base.Comment
	commentIDs []string
	reviews    []*base.Review
}

func (g *GithubGraphQLDownloader) getIssuesGraphQL(ctx context.Context, page, perPage int) ([]*base.Issue, bool, error) {
	if page <= 1 {
		g.issuesCursor = ""
	}
	var resp gqlIssuesResponse
	if err := g.doGraphQLPage(ctx, g.graphQLIssuesQuery(), g.issuesCursor, graphQLPageSize(perPage, githubGraphQLPageSize), &resp); err != nil {
		return nil, false, err
	}
	conn := resp.Repository.Issues
	prefetched := map[string]*gqlPrefetched{}
	issues := make([]*base.Issue, 0, len(conn.Nodes))
	for i := range conn.Nodes {
		issue, err := g.convertGraphQLIssue(ctx, &conn.Nodes[i], prefetched)
		if err != nil {
			return nil, false, err
		}
		issues = append(issues, issue)
	}
	if err := g.enrichGraphQLPage(ctx, prefetched); err != nil {
		return nil, false, err
	}
	g.issuesCursor = conn.PageInfo.EndCursor // only now, a failed page is retried from the same cursor
	return issues, !conn.PageInfo.HasNextPage, nil
}

func (g *GithubGraphQLDownloader) convertGraphQLIssue(ctx context.Context, node *gqlIssue, prefetchedByNodeID map[string]*gqlPrefetched) (*base.Issue, error) {
	labels, err := allNodes(ctx, g, graphQLLabelsSweepQuery, node.ID, node.Labels)
	if err != nil {
		return nil, err
	}
	assignees, err := allNodes(ctx, g, graphQLAssigneesSweepQuery, node.ID, node.Assignees)
	if err != nil {
		return nil, err
	}
	reactions, err := g.allReactions(ctx, node.ID, node.Reactions)
	if err != nil {
		return nil, err
	}
	issue := &base.Issue{
		Number:       node.Number,
		Title:        node.Title,
		Content:      node.Body,
		PosterID:     node.Author.DatabaseID,
		PosterName:   node.Author.Login,
		State:        strings.ToLower(node.State),
		Created:      node.CreatedAt,
		Updated:      node.UpdatedAt,
		Closed:       node.ClosedAt,
		IsLocked:     node.Locked,
		Labels:       make([]*base.Label, 0, len(labels)),
		Reactions:    reactions,
		ForeignIndex: node.Number,
	}
	if node.Milestone != nil {
		issue.Milestone = node.Milestone.Title
	}
	for _, label := range labels {
		issue.Labels = append(issue.Labels, &base.Label{Name: label.Name, Color: label.Color, Description: label.Description})
	}
	for _, assignee := range assignees {
		issue.Assignees = append(issue.Assignees, assignee.Login)
	}
	if g.skipComments {
		return issue, nil
	}

	prefetched := &gqlPrefetched{index: node.Number}
	issue.Context = prefetched
	prefetchedByNodeID[node.ID] = prefetched
	if node.Comments.PageInfo.HasNextPage {
		prefetched.comments, err = g.getComments(ctx, issue)
		return issue, err
	}
	for _, comment := range node.Comments.Nodes {
		prefetched.commentIDs = append(prefetched.commentIDs, comment.ID)
		prefetched.comments = append(prefetched.comments, &base.Comment{
			IssueIndex: node.Number,
			Index:      comment.DatabaseID,
			PosterID:   comment.Author.DatabaseID,
			PosterName: comment.Author.Login,
			Content:    comment.Body,
			Created:    comment.CreatedAt,
			Updated:    comment.UpdatedAt,
		})
	}
	return issue, nil
}

func (g *GithubGraphQLDownloader) enrichGraphQLPage(ctx context.Context, prefetchedByNodeID map[string]*gqlPrefetched) error {
	if !g.SkipReactions {
		comments := map[string]*base.Comment{}
		for _, prefetched := range prefetchedByNodeID {
			for i, id := range prefetched.commentIDs {
				comments[id] = prefetched.comments[i]
			}
		}
		for batch := range slices.Chunk(slices.Sorted(maps.Keys(comments)), graphQLNodeBatchSize) {
			reactions, err := g.fetchNodeReactions(ctx, batch)
			if err != nil {
				return err
			}
			for id, reaction := range reactions {
				comments[id].Reactions = reaction
			}
		}
	}
	return g.attachTimelineEvents(ctx, prefetchedByNodeID)
}

func graphQLPageSize(perPage, maxSize int) int {
	if perPage <= 0 {
		return maxSize
	}
	return min(perPage, maxSize) // Keep pages within the caller's database batch size.
}

func (g *GithubGraphQLDownloader) doGraphQLPage(ctx context.Context, query, cursor string, pageSize int, out any) error {
	vars := map[string]any{"owner": g.repoOwner, "name": g.repoName}
	if cursor != "" {
		vars["cursor"] = cursor
	}
	for {
		vars["first"] = pageSize
		err := g.doGraphQL(ctx, query, vars, out)
		if err == nil || !isGraphQLTooHeavy(err) || pageSize <= 1 {
			return err
		}
		pageSize /= 2
		log.Warn("github graphql: page too heavy (%v), retrying with %d", err, pageSize)
	}
}

func allNodes[T any](ctx context.Context, g *GithubGraphQLDownloader, sweepQuery, nodeID string, conn gqlConn[T]) ([]T, error) {
	nodes := conn.Nodes
	for page := conn.PageInfo; page.HasNextPage; {
		var resp struct {
			Node struct {
				Conn gqlConn[T] `json:"conn"`
			} `json:"node"`
		}
		if err := g.doGraphQL(ctx, sweepQuery, map[string]any{"id": nodeID, "cursor": page.EndCursor}, &resp); err != nil {
			return nil, err
		}
		nodes = append(nodes, resp.Node.Conn.Nodes...)
		page = resp.Node.Conn.PageInfo
	}
	return nodes, nil
}

func (g *GithubGraphQLDownloader) allReactions(ctx context.Context, nodeID string, conn gqlConn[gqlReaction]) ([]*base.Reaction, error) {
	nodes, err := allNodes(ctx, g, graphQLReactionsSweepQuery, nodeID, conn)
	if err != nil || len(nodes) == 0 {
		return nil, err
	}
	reactions := make([]*base.Reaction, 0, len(nodes))
	for _, reaction := range nodes {
		reactions = append(reactions, &base.Reaction{
			UserID:   reaction.User.DatabaseID,
			UserName: reaction.User.Login,
			Content:  graphQLReactionContent(reaction.Content),
		})
	}
	return reactions, nil
}

func (g *GithubGraphQLDownloader) fetchNodeReactions(ctx context.Context, ids []string) (map[string][]*base.Reaction, error) {
	var resp struct {
		Nodes []struct {
			ID        string               `json:"id"`
			Reactions gqlConn[gqlReaction] `json:"reactions"`
		} `json:"nodes"`
	}
	if err := g.doGraphQL(ctx, graphQLNodeReactionsQuery, map[string]any{"ids": ids}, &resp); err != nil {
		if isGraphQLTooHeavy(err) && len(ids) > 1 {
			return fetchHalves(ctx, ids, g.fetchNodeReactions)
		}
		return nil, err
	}
	out := make(map[string][]*base.Reaction, len(resp.Nodes))
	for _, node := range resp.Nodes {
		if node.ID == "" {
			continue
		}
		reactions, err := g.allReactions(ctx, node.ID, node.Reactions)
		if err != nil {
			return nil, err
		}
		out[node.ID] = reactions
	}
	return out, nil
}

func fetchHalves[V any](ctx context.Context, ids []string, fetch func(context.Context, []string) (map[string]V, error)) (map[string]V, error) {
	left, err := fetch(ctx, ids[:len(ids)/2])
	if err != nil {
		return nil, err
	}
	right, err := fetch(ctx, ids[len(ids)/2:])
	if err != nil {
		return nil, err
	}
	maps.Copy(left, right)
	return left, nil
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
