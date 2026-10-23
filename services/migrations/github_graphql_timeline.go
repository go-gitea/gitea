// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
)

// PR-only types in the Issue fragment fail validation of the whole query, assignee events would need local user ids
const (
	issueTimelineItemTypes = "CLOSED_EVENT,REOPENED_EVENT,LOCKED_EVENT,UNLOCKED_EVENT,RENAMED_TITLE_EVENT,LABELED_EVENT,UNLABELED_EVENT,MILESTONED_EVENT,DEMILESTONED_EVENT,PINNED_EVENT,UNPINNED_EVENT"
	prTimelineItemTypes    = issueTimelineItemTypes + ",MERGED_EVENT,HEAD_REF_DELETED_EVENT,AUTO_MERGE_ENABLED_EVENT,AUTO_MERGE_DISABLED_EVENT"
)

const issueTimelineFragments = `
        ... on ClosedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on ReopenedEvent{id createdAt actor{` + gqlActorFields + `}}
        ... on LockedEvent{id createdAt lockReason actor{` + gqlActorFields + `}}
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
}`

const graphQLTimelineSweepQuery = `
query($id:ID!,$cursor:String){
  node(id:$id){
    ... on Issue{conn:timelineItems(first:100,after:$cursor,itemTypes:[` + issueTimelineItemTypes + `])` + issueTimelineConn + `}
    ... on PullRequest{conn:timelineItems(first:100,after:$cursor,itemTypes:[` + prTimelineItemTypes + `])` + prTimelineConn + `}
  }
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
	LockReason     string `json:"lockReason"`
}

func convertTimelineItem(it *gqlTimelineItem) *base.Comment {
	comment := &base.Comment{
		PosterID:   it.Actor.DatabaseID,
		PosterName: it.Actor.Login,
		Created:    it.CreatedAt,
		Updated:    it.CreatedAt,
	}
	switch it.Typename {
	case "ClosedEvent":
		comment.CommentType = issues_model.CommentTypeClose.String()
	case "ReopenedEvent":
		comment.CommentType = issues_model.CommentTypeReopen.String()
	case "LockedEvent":
		comment.CommentType = issues_model.CommentTypeLock.String()
		comment.Content = strings.ToLower(strings.ReplaceAll(it.LockReason, "_", " "))
	case "UnlockedEvent":
		comment.CommentType = issues_model.CommentTypeUnlock.String()
	case "RenamedTitleEvent":
		comment.CommentType = issues_model.CommentTypeChangeTitle.String()
		comment.Meta = map[string]any{"OldTitle": it.PreviousTitle, "NewTitle": it.CurrentTitle}
	case "LabeledEvent", "UnlabeledEvent":
		if it.Label == nil {
			return nil
		}
		comment.CommentType = issues_model.CommentTypeLabel.String()
		comment.Meta = map[string]any{"LabelName": it.Label.Name}
		if it.Typename == "LabeledEvent" {
			comment.Content = "1"
		}
	case "MilestonedEvent":
		comment.CommentType = issues_model.CommentTypeMilestone.String()
		comment.Meta = map[string]any{"MilestoneTitle": it.MilestoneTitle}
	case "DemilestonedEvent":
		comment.CommentType = issues_model.CommentTypeMilestone.String()
		comment.Meta = map[string]any{"MilestoneTitle": it.MilestoneTitle, "Removed": true}
	case "MergedEvent":
		comment.CommentType = issues_model.CommentTypeMergePull.String()
	case "HeadRefDeletedEvent":
		comment.CommentType = issues_model.CommentTypeDeleteBranch.String()
		comment.Content = it.HeadRefName
	case "PinnedEvent":
		comment.CommentType = issues_model.CommentTypePin.String()
	case "UnpinnedEvent":
		comment.CommentType = issues_model.CommentTypeUnpin.String()
	case "AutoMergeEnabledEvent":
		comment.CommentType = issues_model.CommentTypePRScheduledToAutoMerge.String()
	case "AutoMergeDisabledEvent":
		comment.CommentType = issues_model.CommentTypePRUnScheduledToAutoMerge.String()
	default:
		log.Warn("github graphql: dropping unhandled timeline event type %q (node %s)", it.Typename, it.ID)
		return nil
	}
	return comment
}

func convertTimelineItems(items []gqlTimelineItem) []*base.Comment {
	comments := make([]*base.Comment, 0, len(items))
	for i := range items {
		if comment := convertTimelineItem(&items[i]); comment != nil {
			comments = append(comments, comment)
		}
	}
	return comments
}

func (g *GithubGraphQLDownloader) attachTimelineEvents(ctx context.Context, prefetchedByNodeID map[string]*gqlPrefetched) error {
	for batch := range slices.Chunk(slices.Sorted(maps.Keys(prefetchedByNodeID)), graphQLNodeBatchSize) {
		eventsByID, err := g.fetchTimelineEvents(ctx, batch)
		if err != nil {
			return err
		}
		for id, events := range eventsByID {
			prefetched := prefetchedByNodeID[id]
			for _, event := range events {
				event.IssueIndex = prefetched.index
			}
			prefetched.comments = append(prefetched.comments, events...)
		}
	}
	return nil
}

func (g *GithubGraphQLDownloader) fetchTimelineEvents(ctx context.Context, ids []string) (map[string][]*base.Comment, error) {
	var resp struct {
		Nodes []struct {
			ID            string                   `json:"id"`
			TimelineItems gqlConn[gqlTimelineItem] `json:"timelineItems"`
		} `json:"nodes"`
	}
	if err := g.doGraphQL(ctx, graphQLTimelineBatchQuery, map[string]any{"ids": ids}, &resp); err != nil {
		if isGraphQLTooHeavy(err) && len(ids) > 1 {
			return fetchHalves(ctx, ids, g.fetchTimelineEvents)
		}
		return nil, err
	}
	out := make(map[string][]*base.Comment, len(resp.Nodes))
	for _, node := range resp.Nodes {
		if node.ID == "" {
			continue
		}
		items, err := allNodes(ctx, g, graphQLTimelineSweepQuery, node.ID, node.TimelineItems)
		if err != nil {
			return nil, err
		}
		out[node.ID] = dropCloseOnMerge(convertTimelineItems(items))
	}
	return out, nil
}

// dropCloseOnMerge drops the close GitHub records with a merge, the last one since a merged PR cannot be reopened
func dropCloseOnMerge(events []*base.Comment) []*base.Comment {
	if !slices.ContainsFunc(events, func(event *base.Comment) bool { return event.CommentType == issues_model.CommentTypeMergePull.String() }) {
		return events
	}
	for i, event := range slices.Backward(events) {
		switch event.CommentType {
		case issues_model.CommentTypeReopen.String():
			return events
		case issues_model.CommentTypeClose.String():
			return slices.Delete(events, i, i+1)
		}
	}
	return events
}
