// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"strings"
	"testing"

	"gitea.dev/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertTimelineItems parses a timeline page (the shape of #13603's real
// timeline plus milestone/rename) and checks each event maps to the right Gitea
// comment type, meta and actor — and that unmapped types are dropped.
func TestConvertTimelineItems(t *testing.T) {
	const sample = `{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[
      {"__typename":"ClosedEvent","id":"c1","createdAt":"2020-11-17T15:54:33Z","actor":{"login":"jolheiser","databaseId":42128690}},
      {"__typename":"LabeledEvent","id":"l1","createdAt":"2020-11-17T15:54:38Z","label":{"name":"type/question"},"actor":{"login":"jolheiser"}},
      {"__typename":"UnlabeledEvent","id":"u1","createdAt":"2020-11-18T00:00:00Z","label":{"name":"type/bug"},"actor":{"login":"jolheiser"}},
      {"__typename":"LockedEvent","id":"k1","createdAt":"2020-11-17T16:31:00Z","actor":{"login":"go-gitea"}},
      {"__typename":"RenamedTitleEvent","id":"r1","createdAt":"2020-11-19T00:00:00Z","previousTitle":"old","currentTitle":"new","actor":{"login":"x"}},
      {"__typename":"MilestonedEvent","id":"m1","createdAt":"2020-11-20T00:00:00Z","milestoneTitle":"v1.14","actor":{"login":"y"}},
      {"__typename":"DemilestonedEvent","id":"d1","createdAt":"2020-11-21T00:00:00Z","milestoneTitle":"v1.13","actor":{"login":"z"}},
      {"__typename":"MergedEvent","id":"mg1","createdAt":"2020-11-22T00:00:00Z","actor":{"login":"a"}},
      {"__typename":"HeadRefDeletedEvent","id":"hd1","createdAt":"2020-11-22T01:00:00Z","headRefName":"feature/x","actor":{"login":"b"}},
      {"__typename":"PinnedEvent","id":"pn1","createdAt":"2020-11-22T02:00:00Z","actor":{"login":"c"}},
      {"__typename":"UnpinnedEvent","id":"up1","createdAt":"2020-11-22T03:00:00Z","actor":{"login":"d"}},
      {"__typename":"AutoMergeEnabledEvent","id":"am1","createdAt":"2020-11-22T04:00:00Z","actor":{"login":"e"}},
      {"__typename":"AutoMergeDisabledEvent","id":"ad1","createdAt":"2020-11-22T05:00:00Z","actor":{"login":"f"}},
      {"__typename":"MentionedEvent","id":"skip"}
    ]}`
	var conn gqlTimelineConn
	require.NoError(t, json.Unmarshal([]byte(sample), &conn))
	events := convertTimelineItems(conn.Nodes)
	require.Len(t, events, 13) // MentionedEvent dropped

	assert.Equal(t, "close", events[0].CommentType)
	assert.Equal(t, "jolheiser", events[0].PosterName)
	assert.EqualValues(t, 42128690, events[0].PosterID)

	assert.Equal(t, "label", events[1].CommentType)
	assert.Equal(t, "1", events[1].Content) // add
	assert.Equal(t, "type/question", events[1].Meta["LabelName"])

	assert.Equal(t, "label", events[2].CommentType)
	assert.Empty(t, events[2].Content) // remove
	assert.Equal(t, "type/bug", events[2].Meta["LabelName"])

	assert.Equal(t, "lock", events[3].CommentType)

	assert.Equal(t, "change_title", events[4].CommentType)
	assert.Equal(t, "old", events[4].Meta["OldTitle"])
	assert.Equal(t, "new", events[4].Meta["NewTitle"])

	assert.Equal(t, "milestone", events[5].CommentType)
	assert.Equal(t, "v1.14", events[5].Meta["MilestoneTitle"])
	assert.Nil(t, events[5].Meta["Removed"])

	assert.Equal(t, "milestone", events[6].CommentType)
	assert.Equal(t, true, events[6].Meta["Removed"])

	assert.Equal(t, "merge_pull", events[7].CommentType)
	assert.Equal(t, "delete_branch", events[8].CommentType)
	assert.Equal(t, "feature/x", events[8].Content) // deleted branch name
	assert.Equal(t, "pin", events[9].CommentType)
	assert.Equal(t, "unpin", events[10].CommentType)
	assert.Equal(t, "pull_scheduled_merge", events[11].CommentType)
	assert.Equal(t, "pull_cancel_scheduled_merge", events[12].CommentType)

	// every event carries its real timestamp; Gitea sorts the timeline by it
	for _, e := range events {
		assert.False(t, e.Created.IsZero(), "event must carry its GitHub timestamp")
	}
}

// TestTimelineQueryShape guards the fix for #27's class of bug: the timeline query
// is a node-id pass (≤2 levels), never nested in the content query, and carries no
// reactions.
func TestTimelineQueryShape(t *testing.T) {
	batch := graphQLTimelineBatchQuery()
	assert.Contains(t, batch, "nodes(ids:$ids)")
	assert.Contains(t, batch, "... on Issue{timelineItems")
	assert.Contains(t, batch, "... on PullRequest{timelineItems")
	assert.Contains(t, batch, "CLOSED_EVENT")
	assert.Contains(t, batch, "LABELED_EVENT")
	assert.Contains(t, batch, "MILESTONED_EVENT")
	assert.Contains(t, batch, "... on ClosedEvent")
	assert.NotContains(t, batch, "reactions", "timeline query must not carry reactions")

	sweep := graphQLTimelineSweepQuery()
	assert.Contains(t, sweep, "node(id:$id)")
	assert.Contains(t, sweep, "after:$cursor")

	// The timeline queries must be syntactically valid — brace-balanced. A missing
	// close in timelineConnFields made the whole query malformed (Expected NAME at
	// [35,1]), which aborted the entire metadata import (#37).
	for name, q := range map[string]string{"batch": batch, "sweep": sweep} {
		assert.Equal(t, strings.Count(q, "{"), strings.Count(q, "}"), "%s timeline query must be brace-balanced", name)
	}
}

// TestGraphQLQueriesFetchNodeID guards #28: the issue/PR node id must be selected,
// or node.ID is empty, the timeline (and reaction) node-id passes never run, and
// the feature silently no-ops.
func TestGraphQLQueriesFetchNodeID(t *testing.T) {
	g := &GithubDownloaderV3{}
	assert.Contains(t, g.graphQLIssuesQuery(), "\n        id number ", "issue query must select the node id")
	assert.Contains(t, g.graphQLPullRequestsQuery(), "\n        id number ", "PR query must select the node id")
}
