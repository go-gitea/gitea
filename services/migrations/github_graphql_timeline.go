// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

// Timeline events are fetched by a separate node-id pass: nested in the content query they exceed GitHub's per-query
// compute budget. Assignee events are omitted since a Gitea assignee comment needs a local user id.

import (
	"context"
	"maps"
	"slices"
	"time"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
)

const timelineBatchSize = 100

// PR-only types in the Issue union are a validation error that aborts the whole pass
const (
	issueTimelineItemTypes = "CLOSED_EVENT,REOPENED_EVENT,LOCKED_EVENT,UNLOCKED_EVENT,RENAMED_TITLE_EVENT,LABELED_EVENT,UNLABELED_EVENT,MILESTONED_EVENT,DEMILESTONED_EVENT,PINNED_EVENT,UNPINNED_EVENT"
	prTimelineItemTypes    = issueTimelineItemTypes + ",MERGED_EVENT,HEAD_REF_DELETED_EVENT,AUTO_MERGE_ENABLED_EVENT,AUTO_MERGE_DISABLED_EVENT"
)

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

const (
	issueTimelineConn = `{
      pageInfo{hasNextPage endCursor}
      nodes{
        __typename` + issueTimelineFragments + `
      }
    }`
	prTimelineConn = `{
      pageInfo{hasNextPage endCursor}
      nodes{
        __typename` + prTimelineFragments + `
      }
    }`
)

const graphQLTimelineBatchQuery = `
query($ids:[ID!]!){
  nodes(ids:$ids){
    id
    ... on Issue{timelineItems(first:100,itemTypes:[` + issueTimelineItemTypes + `])` + issueTimelineConn + `}
    ... on PullRequest{timelineItems(first:100,itemTypes:[` + prTimelineItemTypes + `])` + prTimelineConn + `}
  }
  rateLimit{cost remaining resetAt}
}`

const graphQLTimelineSweepQuery = `
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on Issue{conn:timelineItems(first:100,after:$cursor,itemTypes:[` + issueTimelineItemTypes + `])` + issueTimelineConn + `}
    ... on PullRequest{conn:timelineItems(first:100,after:$cursor,itemTypes:[` + prTimelineItemTypes + `])` + prTimelineConn + `}
  }
  rateLimit{cost remaining resetAt}
}`

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

// convertTimelineItem returns nil for events without a usable payload
func convertTimelineItem(it *gqlTimelineItem) *base.Comment {
	c := &base.Comment{
		PosterID:   it.Actor.DatabaseID,
		PosterName: it.Actor.Login,
		Created:    it.CreatedAt,
		Updated:    it.CreatedAt,
	}
	switch it.Typename {
	case "ClosedEvent":
		c.CommentType = issues_model.CommentTypeClose.String()
	case "ReopenedEvent":
		c.CommentType = issues_model.CommentTypeReopen.String()
	case "LockedEvent":
		c.CommentType = issues_model.CommentTypeLock.String()
	case "UnlockedEvent":
		c.CommentType = issues_model.CommentTypeUnlock.String()
	case "RenamedTitleEvent":
		c.CommentType = issues_model.CommentTypeChangeTitle.String()
		c.Meta = map[string]any{"OldTitle": it.PreviousTitle, "NewTitle": it.CurrentTitle}
	case "LabeledEvent":
		if it.Label == nil {
			return nil
		}
		c.CommentType = issues_model.CommentTypeLabel.String()
		c.Content = "1" // Gitea marks an add with Content "1", a remove with ""
		c.Meta = map[string]any{"LabelName": it.Label.Name}
	case "UnlabeledEvent":
		if it.Label == nil {
			return nil
		}
		c.CommentType = issues_model.CommentTypeLabel.String()
		c.Meta = map[string]any{"LabelName": it.Label.Name}
	case "MilestonedEvent":
		c.CommentType = issues_model.CommentTypeMilestone.String()
		c.Meta = map[string]any{"MilestoneTitle": it.MilestoneTitle}
	case "DemilestonedEvent":
		c.CommentType = issues_model.CommentTypeMilestone.String()
		c.Meta = map[string]any{"MilestoneTitle": it.MilestoneTitle, "Removed": true}
	case "MergedEvent":
		c.CommentType = issues_model.CommentTypeMergePull.String()
	case "HeadRefDeletedEvent":
		c.CommentType = issues_model.CommentTypeDeleteBranch.String()
		c.Content = it.HeadRefName
	case "PinnedEvent":
		c.CommentType = issues_model.CommentTypePin.String()
	case "UnpinnedEvent":
		c.CommentType = issues_model.CommentTypeUnpin.String()
	case "AutoMergeEnabledEvent":
		c.CommentType = issues_model.CommentTypePRScheduledToAutoMerge.String()
	case "AutoMergeDisabledEvent":
		c.CommentType = issues_model.CommentTypePRUnScheduledToAutoMerge.String()
	default:
		// the itemTypes list and this switch drifted apart
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

// attachTimelineEvents appends the events to the comment cache; Gitea sorts the timeline by created time.
func (g *GithubDownloaderV3) attachTimelineEvents(ctx context.Context, nodeIDToIndex map[string]int64) error {
	if len(nodeIDToIndex) == 0 {
		return nil
	}
	log.Info("metadata sync [%s/%s]: timeline — fetching events for %d issues/PRs",
		g.repoOwner, g.repoName, len(nodeIDToIndex))
	for batch := range slices.Chunk(sortedNodeIDs(nodeIDToIndex), timelineBatchSize) {
		eventsByID, err := g.fetchTimelineEvents(ctx, batch)
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
	if err := g.doGraphQL(ctx, graphQLTimelineBatchQuery, map[string]any{"ids": ids}, &resp); err != nil {
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
	g.recordGraphQLBudget(resp.RateLimit)

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
		out[n.ID] = dropCloseOnMerge(events)
	}
	return out, nil
}

func (g *GithubDownloaderV3) sweepTimelineEvents(ctx context.Context, nodeID, cursor string) ([]*base.Comment, error) {
	items, err := sweepNodeConnection[gqlTimelineItem](ctx, g, graphQLTimelineSweepQuery, nodeID, cursor)
	if err != nil {
		return nil, err
	}
	return convertTimelineItems(items), nil
}

// a close further from the merge is a genuine earlier close
const mergeCloseWindow = time.Minute

// dropCloseOnMerge drops GitHub's ClosedEvent next to every merge: Gitea writes only a merge comment.
func dropCloseOnMerge(events []*base.Comment) []*base.Comment {
	var merges []time.Time
	for _, e := range events {
		if e.CommentType == issues_model.CommentTypeMergePull.String() {
			merges = append(merges, e.Created)
		}
	}
	if len(merges) == 0 {
		return events
	}
	return slices.DeleteFunc(events, func(e *base.Comment) bool {
		return e.CommentType == issues_model.CommentTypeClose.String() && slices.ContainsFunc(merges, func(m time.Time) bool {
			return e.Created.Sub(m).Abs() <= mergeCloseWindow
		})
	})
}
