// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

// Issue/PR timeline events (closed, reopened, locked, unlocked, renamed, labeled,
// milestoned) are imported as typed Gitea comments so the mirror shows issue
// history — who closed/labeled/locked and when — not just the final state.
//
// They are fetched by a batched node-id pass (like reactions, #27), NOT nested in
// the content query: a 3rd connection level under issues/PRs blows GitHub's
// per-query compute budget (RESOURCE_LIMITS_EXCEEDED, #27). Unlike reactions,
// timeline events bump the issue's updated_at, so a new event re-surfaces the issue
// in the normal watermark sync — no periodic rescan — and they are immutable and
// append-only, so no removal reconcile. Gitea renders the timeline sorted by
// created_unix across comments and typed events, so fetch order is irrelevant as
// long as each event carries its real event timestamp.
//
// Assignee events are intentionally omitted: a Gitea assignee comment references a
// user by id with no name fallback, and migrated GitHub users are not Gitea users.

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"slices"
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
// query adds the PR-only types on top.
const (
	issueTimelineItemTypes = "CLOSED_EVENT,REOPENED_EVENT,LOCKED_EVENT,UNLOCKED_EVENT,RENAMED_TITLE_EVENT,LABELED_EVENT,UNLABELED_EVENT,MILESTONED_EVENT,DEMILESTONED_EVENT,PINNED_EVENT,UNPINNED_EVENT"
	prTimelineItemTypes    = issueTimelineItemTypes + ",MERGED_EVENT,HEAD_REF_DELETED_EVENT,AUTO_MERGE_ENABLED_EVENT,AUTO_MERGE_DISABLED_EVENT"
)

// Inline-fragment selections, likewise split. These event types expose no databaseId,
// only the node id.
const issueTimelineFragments = `
        ... on ClosedEvent{id createdAt actor{login ... on User{databaseId}}}
        ... on ReopenedEvent{id createdAt actor{login ... on User{databaseId}}}
        ... on LockedEvent{id createdAt actor{login ... on User{databaseId}}}
        ... on UnlockedEvent{id createdAt actor{login ... on User{databaseId}}}
        ... on RenamedTitleEvent{id createdAt previousTitle currentTitle actor{login ... on User{databaseId}}}
        ... on LabeledEvent{id createdAt label{name} actor{login ... on User{databaseId}}}
        ... on UnlabeledEvent{id createdAt label{name} actor{login ... on User{databaseId}}}
        ... on MilestonedEvent{id createdAt milestoneTitle actor{login ... on User{databaseId}}}
        ... on DemilestonedEvent{id createdAt milestoneTitle actor{login ... on User{databaseId}}}
        ... on PinnedEvent{id createdAt actor{login ... on User{databaseId}}}
        ... on UnpinnedEvent{id createdAt actor{login ... on User{databaseId}}}`

const prTimelineFragments = issueTimelineFragments + `
        ... on MergedEvent{id createdAt actor{login ... on User{databaseId}}}
        ... on HeadRefDeletedEvent{id createdAt headRefName actor{login ... on User{databaseId}}}
        ... on AutoMergeEnabledEvent{id createdAt actor{login ... on User{databaseId}}}
        ... on AutoMergeDisabledEvent{id createdAt actor{login ... on User{databaseId}}}`

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
func graphQLTimelineBatchQuery() string {
	return fmt.Sprintf(`
query($ids:[ID!]!){
  nodes(ids:$ids){
    id
    ... on Issue{timelineItems(first:100,itemTypes:[%[1]s])%[2]s}
    ... on PullRequest{timelineItems(first:100,itemTypes:[%[3]s])%[4]s}
  }
  rateLimit{cost remaining resetAt}
}`, issueTimelineItemTypes, timelineConn(issueTimelineFragments), prTimelineItemTypes, timelineConn(prTimelineFragments))
}

// graphQLTimelineSweepQuery pages the remaining timeline of a single hot issue/PR
// (one with more than one page of events), by node id.
func graphQLTimelineSweepQuery() string {
	return fmt.Sprintf(`
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on Issue{timelineItems(first:100,after:$cursor,itemTypes:[%[1]s])%[2]s}
    ... on PullRequest{timelineItems(first:100,after:$cursor,itemTypes:[%[3]s])%[4]s}
  }
  rateLimit{cost remaining resetAt}
}`, issueTimelineItemTypes, timelineConn(issueTimelineFragments), prTimelineItemTypes, timelineConn(prTimelineFragments))
}

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
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
	Nodes []gqlTimelineItem `json:"nodes"`
}

// timelineEventID hashes a timeline event's GraphQL node id (these events carry no
// databaseId) to a stable positive int64 for the OriginalID dedup key, so a re-sync
// upserts each event rather than duplicating it.
func timelineEventID(nodeID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(nodeID))
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
}

// convertTimelineItem maps one timeline event to a typed base.Comment. Returns nil
// for events that carry no usable payload (e.g. a label event with no label).
func convertTimelineItem(it *gqlTimelineItem) *base.Comment {
	c := &base.Comment{
		Index:      timelineEventID(it.ID),
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
// comment cache, so the comment phase persists them interleaved with regular
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
		var ge graphQLError
		if errors.As(err, &ge) && ge.Type == "RESOURCE_LIMITS_EXCEEDED" && len(ids) > 1 {
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

// sweepTimelineEvents pages the remaining timeline of one node over GraphQL,
// following the cursor.
func (g *GithubDownloaderV3) sweepTimelineEvents(ctx context.Context, nodeID, cursor string) ([]*base.Comment, error) {
	var all []*base.Comment
	for {
		vars := map[string]any{"id": nodeID, "cursor": cursor}
		var resp struct {
			Node struct {
				TimelineItems gqlTimelineConn `json:"timelineItems"`
			} `json:"node"`
			RateLimit graphQLRateLimit `json:"rateLimit"`
		}
		if err := g.doGraphQL(ctx, graphQLTimelineSweepQuery(), vars, &resp); err != nil {
			return nil, err
		}
		g.respectGraphQLBudget(ctx, resp.RateLimit)
		all = append(all, convertTimelineItems(resp.Node.TimelineItems.Nodes)...)
		if !resp.Node.TimelineItems.PageInfo.HasNextPage {
			return all, nil
		}
		cursor = resp.Node.TimelineItems.PageInfo.EndCursor
	}
}
