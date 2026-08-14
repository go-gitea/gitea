// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

// Issue/PR timeline events (closed, reopened, locked, unlocked, renamed, labeled,
// milestoned) are imported as typed Gitea comments so the mirror shows issue
// history — who closed/labeled/locked and when — not just the final state.
//
// They are fetched by a batched node-id pass (like reactions, #27), NOT nested in
// the content query: a 3rd connection level under issues/PRs blows GitHub's
// per-query compute budget (RESOURCE_LIMITS_EXCEEDED, #27). Gitea renders the
// timeline sorted by created_unix across comments and typed events, so fetch order
// is irrelevant as long as each event carries its real event timestamp.
//
// An event whose target cannot be resolved locally is dropped by the uploader
// rather than stored half-formed: see the label/milestone cases in CreateComments.
//
// Assignee events are intentionally omitted: a Gitea assignee comment references a
// user by id with no name fallback, and migrated GitHub users are not Gitea users.

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
)

// timelineBatchSize is how many issue/PR node ids are asked for their timeline per
// request.
const timelineBatchSize = 100

// Timeline events must be split by union: MERGED/HEAD_REF_DELETED/AUTO_MERGE_* are
// PullRequest-only — including them in the Issue timelineItems (itemTypes arg OR an
// `... on MergedEvent` fragment) is a hard GraphQL validation error that aborts the
// whole timeline pass. issueTimelineItemTypes is the set valid on BOTH unions; the PR
// query adds the PR-only types on top. The complete lists of accepted values are the
// IssueTimelineItemsItemType and PullRequestTimelineItemsItemType enums:
// https://docs.github.com/en/graphql/reference/enums#issuetimelineitemsitemtype
// https://docs.github.com/en/graphql/reference/enums#pullrequesttimelineitemsitemtype
// The types handled here are the subset with a Gitea comment-type equivalent.
const (
	issueTimelineItemTypes = "CLOSED_EVENT,REOPENED_EVENT,LOCKED_EVENT,UNLOCKED_EVENT,RENAMED_TITLE_EVENT,LABELED_EVENT,UNLABELED_EVENT,MILESTONED_EVENT,DEMILESTONED_EVENT,PINNED_EVENT,UNPINNED_EVENT"
	prTimelineItemTypes    = issueTimelineItemTypes + ",MERGED_EVENT,HEAD_REF_DELETED_EVENT,AUTO_MERGE_ENABLED_EVENT,AUTO_MERGE_DISABLED_EVENT"
)

// Inline-fragment selections, likewise split. These event types expose no databaseId,
// only the node id; their actor's id comes from gqlActorFields.
const issueTimelineFragments = `
        ... on ClosedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on ReopenedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on LockedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on UnlockedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on RenamedTitleEvent{id createdAt previousTitle currentTitle actor{` + gqlActorFields + `}}
        ... on LabeledEvent{id createdAt label{name} actor{` + gqlActorFields + `}}
        ... on UnlabeledEvent{id createdAt label{name} actor{` + gqlActorFields + `}}
        ... on MilestonedEvent{id createdAt milestoneTitle actor{` + gqlActorFields + `}}
        ... on DemilestonedEvent{id createdAt milestoneTitle actor{` + gqlActorFields + `}}
        ... on PinnedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on UnpinnedEvent{id createdAt actor{` + gqlActorFields + `}}`

const prTimelineFragments = issueTimelineFragments + `
        ... on MergedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on HeadRefDeletedEvent{id createdAt headRefName actor{` + gqlActorFields + `}}
        ... on AutoMergeEnabledEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on AutoMergeDisabledEvent{id createdAt actor{` + gqlActorFields + `}}`

// timelineConn wraps a fragment selection in the shared connection envelope.
func timelineConn(fragments string) string {
	return `{
      pageInfo{hasNextPage endCursor}
      nodes{
        __typename` + fragments + `
      }
    }`
}

// graphQLTimelineBatchQuery fetches the first page of timeline events for a batch of
// issue/PR node ids in one request (two levels deep). Issue and PullRequest both
// expose timelineItems, and the response merges to the same `timelineItems` key.
// Built once — the query is constant, and a sweep asks for it per batch.
var graphQLTimelineBatchQuery = sync.OnceValue(func() string {
	return fmt.Sprintf(`
query($ids:[ID!]!){
  nodes(ids:$ids){
    id
    ... on Issue{timelineItems(first:100,itemTypes:[%[1]s])%[2]s}
    ... on PullRequest{timelineItems(first:100,itemTypes:[%[3]s])%[4]s}
  }
  rateLimit{cost remaining resetAt}
}`, issueTimelineItemTypes, timelineConn(issueTimelineFragments), prTimelineItemTypes, timelineConn(prTimelineFragments))
})

// graphQLTimelineSweepQuery pages the remaining timeline of a single hot issue/PR
// (one with more than one page of events), by node id. The connection is aliased to
// `conn` so the shared single-node sweep drives it (see sweepNodeConnection).
var graphQLTimelineSweepQuery = sync.OnceValue(func() string {
	return fmt.Sprintf(`
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on Issue{conn:timelineItems(first:100,after:$cursor,itemTypes:[%[1]s])%[2]s}
    ... on PullRequest{conn:timelineItems(first:100,after:$cursor,itemTypes:[%[3]s])%[4]s}
  }
  rateLimit{cost remaining resetAt}
}`, issueTimelineItemTypes, timelineConn(issueTimelineFragments), prTimelineItemTypes, timelineConn(prTimelineFragments))
})

type gqlTimelineItem struct {
	Typename      string    `json:"__typename"`
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"createdAt"`
	Actor         gqlActor  `json:"actor"`
	PreviousTitle string    `json:"previousTitle"`
	CurrentTitle  string    `json:"currentTitle"`
	Label         *struct {
		Name string `json:"name"`
	} `json:"label"`
	MilestoneTitle string `json:"milestoneTitle"`
	HeadRefName    string `json:"headRefName"`
}

type gqlTimelineConn struct {
	PageInfo gqlPageInfo       `json:"pageInfo"`
	Nodes    []gqlTimelineItem `json:"nodes"`
}

// convertTimelineItem maps one timeline event to a typed base.Comment. Returns nil
// for events that carry no usable payload (e.g. a label event with no label).
//
// Index is deliberately left unset: the uploader builds its comment rows from the
// type, content and timestamps only, so there is nowhere for a remote id to land
// and no upsert to key off.
func convertTimelineItem(it *gqlTimelineItem) *base.Comment {
	c := &base.Comment{
		PosterID:   it.Actor.DatabaseID,
		PosterName: it.Actor.Login,
		Created:    it.CreatedAt,
		Updated:    it.CreatedAt,
	}
	switch it.Typename {
	case "ClosedEvent":
		c.CommentType = "close"
	case "ReopenedEvent":
		c.CommentType = "reopen"
	case "LockedEvent":
		c.CommentType = "lock"
	case "UnlockedEvent":
		c.CommentType = "unlock"
	case "RenamedTitleEvent":
		c.CommentType = "change_title"
		c.Meta = map[string]any{"OldTitle": it.PreviousTitle, "NewTitle": it.CurrentTitle}
	case "LabeledEvent":
		if it.Label == nil {
			return nil
		}
		c.CommentType = "label"
		c.Content = "1" // Gitea marks an add with Content "1", a remove with ""
		c.Meta = map[string]any{"LabelName": it.Label.Name}
	case "UnlabeledEvent":
		if it.Label == nil {
			return nil
		}
		c.CommentType = "label"
		c.Meta = map[string]any{"LabelName": it.Label.Name}
	case "MilestonedEvent":
		c.CommentType = "milestone"
		c.Meta = map[string]any{"MilestoneTitle": it.MilestoneTitle}
	case "DemilestonedEvent":
		c.CommentType = "milestone"
		c.Meta = map[string]any{"MilestoneTitle": it.MilestoneTitle, "Removed": true}
	case "MergedEvent":
		c.CommentType = "merge_pull"
	case "HeadRefDeletedEvent":
		c.CommentType = "delete_branch"
		c.Content = it.HeadRefName
	case "PinnedEvent":
		c.CommentType = "pin"
	case "UnpinnedEvent":
		c.CommentType = "unpin"
	case "AutoMergeEnabledEvent":
		c.CommentType = "pull_scheduled_merge"
	case "AutoMergeDisabledEvent":
		c.CommentType = "pull_cancel_scheduled_merge"
	default:
		// Normally unreachable: the query's itemTypes argument only requests the
		// types handled above. Reaching this means the itemTypes list and this
		// switch have drifted apart — log it so the dropped event is visible.
		log.Warn("github graphql: dropping unhandled timeline event type %q (node %s)", it.Typename, it.ID)
		return nil
	}
	return c
}

func convertTimelineItems(items []gqlTimelineItem) []*base.Comment {
	comments := make([]*base.Comment, 0, len(items))
	for i := range items {
		if c := convertTimelineItem(&items[i]); c != nil {
			comments = append(comments, c)
		}
	}
	return comments
}

// attachTimelineEvents fetches timeline events for the given issues/PRs (keyed by
// GraphQL node id → local index) and appends them as typed comments to the shared
// comment cache, so GetComments hands them over interleaved with regular
// comments. No-op when the map is empty.
func (g *GithubDownloaderV3) attachTimelineEvents(ctx context.Context, nodeIDToIndex map[string]int64) error {
	if len(nodeIDToIndex) == 0 {
		return nil
	}
	log.Info("metadata sync [%s/%s]: timeline — fetching events for %d issues/PRs",
		g.repoOwner, g.repoName, len(nodeIDToIndex))
	ids := make([]string, 0, len(nodeIDToIndex))
	for id := range nodeIDToIndex {
		if id != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids) // deterministic batching
	for start := 0; start < len(ids); start += timelineBatchSize {
		end := min(start+timelineBatchSize, len(ids))
		eventsByID, err := g.fetchTimelineEvents(ctx, ids[start:end])
		if err != nil {
			return err
		}
		for id, events := range eventsByID {
			index := nodeIDToIndex[id]
			for _, e := range events {
				e.IssueIndex = index
			}
			g.gqlComments[index] = append(g.gqlComments[index], events...)
		}
	}
	return nil
}

// fetchTimelineEvents returns timeline events for a batch of issue/PR node ids. On
// RESOURCE_LIMITS_EXCEEDED it halves the batch and retries (adaptive shrink) rather
// than aborting; a hot entity with more than one page sweeps the remainder.
func (g *GithubDownloaderV3) fetchTimelineEvents(ctx context.Context, ids []string) (map[string][]*base.Comment, error) {
	if len(ids) == 0 {
		return map[string][]*base.Comment{}, nil
	}
	var resp struct {
		Nodes []struct {
			ID            string          `json:"id"`
			TimelineItems gqlTimelineConn `json:"timelineItems"`
		} `json:"nodes"`
		RateLimit graphQLRateLimit `json:"rateLimit"`
	}
	if err := g.doGraphQL(ctx, graphQLTimelineBatchQuery(), map[string]any{"ids": ids}, &resp); err != nil {
		if isGraphQLResourceLimited(err) && len(ids) > 1 {
			mid := len(ids) / 2
			left, err := g.fetchTimelineEvents(ctx, ids[:mid])
			if err != nil {
				return nil, err
			}
			right, err := g.fetchTimelineEvents(ctx, ids[mid:])
			if err != nil {
				return nil, err
			}
			maps.Copy(left, right)

			return left, nil
		}
		return nil, err
	}
	g.respectGraphQLBudget(ctx, resp.RateLimit)

	out := make(map[string][]*base.Comment, len(resp.Nodes))
	for i := range resp.Nodes {
		n := &resp.Nodes[i]
		if n.ID == "" {
			continue // null node (unresolvable id)
		}
		events := convertTimelineItems(n.TimelineItems.Nodes)
		if n.TimelineItems.PageInfo.HasNextPage {
			rest, err := g.sweepTimelineEvents(ctx, n.ID, n.TimelineItems.PageInfo.EndCursor)
			if err != nil {
				return nil, err
			}
			events = append(events, rest...)
		}
		out[n.ID] = events
	}
	return out, nil
}

// sweepTimelineEvents pages the timeline of one node from cursor to its end.
func (g *GithubDownloaderV3) sweepTimelineEvents(ctx context.Context, nodeID, cursor string) ([]*base.Comment, error) {
	items, err := sweepNodeConnection[gqlTimelineItem](ctx, g, graphQLTimelineSweepQuery(), nodeID, cursor)
	if err != nil {
		return nil, err
	}
	return convertTimelineItems(items), nil
}
