// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"testing"
	"time"

	"gitea.dev/modules/json"
	base "gitea.dev/modules/migration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertTimelineItems(t *testing.T) {
	for _, testcase := range []struct {
		name  string
		event string
		want  *base.Comment
	}{
		{"close by user", `{"__typename":"ClosedEvent","actor":{"login":"closer","databaseId":42}}`, &base.Comment{CommentType: "close", PosterName: "closer", PosterID: 42}},
		{"reopen", `{"__typename":"ReopenedEvent"}`, &base.Comment{CommentType: "reopen"}},
		{"label added", `{"__typename":"LabeledEvent","label":{"name":"bug"}}`, &base.Comment{CommentType: "label", Content: "1", Meta: map[string]any{"LabelName": "bug"}}},
		{"label removed", `{"__typename":"UnlabeledEvent","label":{"name":"bug"}}`, &base.Comment{CommentType: "label", Meta: map[string]any{"LabelName": "bug"}}},
		{"deleted label added is skipped", `{"__typename":"LabeledEvent","label":null}`, nil},
		{"deleted label removed is skipped", `{"__typename":"UnlabeledEvent","label":null}`, nil},
		{"lock reason in content", `{"__typename":"LockedEvent","lockReason":"TOO_HEATED"}`, &base.Comment{CommentType: "lock", Content: "too heated"}},
		{"unlock", `{"__typename":"UnlockedEvent"}`, &base.Comment{CommentType: "unlock"}},
		{"title changed", `{"__typename":"RenamedTitleEvent","previousTitle":"old","currentTitle":"new"}`, &base.Comment{CommentType: "change_title", Meta: map[string]any{"OldTitle": "old", "NewTitle": "new"}}},
		{"milestone added", `{"__typename":"MilestonedEvent","milestoneTitle":"v1"}`, &base.Comment{CommentType: "milestone", Meta: map[string]any{"MilestoneTitle": "v1"}}},
		{"milestone removed", `{"__typename":"DemilestonedEvent","milestoneTitle":"v1"}`, &base.Comment{CommentType: "milestone", Meta: map[string]any{"MilestoneTitle": "v1", "Removed": true}}},
		{"merge by bot", `{"__typename":"MergedEvent","actor":{"__typename":"Bot","login":"merger","databaseId":88}}`, &base.Comment{CommentType: "merge_pull", PosterName: "merger[bot]", PosterID: 88}},
		{"branch deleted", `{"__typename":"HeadRefDeletedEvent","headRefName":"feature"}`, &base.Comment{CommentType: "delete_branch", Content: "feature"}},
		{"pin", `{"__typename":"PinnedEvent"}`, &base.Comment{CommentType: "pin"}},
		{"null actor becomes ghost", `{"__typename":"UnpinnedEvent","actor":null}`, &base.Comment{CommentType: "unpin", PosterName: "ghost"}},
		{"auto merge enabled", `{"__typename":"AutoMergeEnabledEvent"}`, &base.Comment{CommentType: "pull_scheduled_merge"}},
		{"auto merge disabled", `{"__typename":"AutoMergeDisabledEvent"}`, &base.Comment{CommentType: "pull_cancel_scheduled_merge"}},
		{"unknown event is skipped", `{"__typename":"MentionedEvent"}`, nil},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			var item gqlTimelineItem
			require.NoError(t, json.Unmarshal([]byte(testcase.event), &item))
			item.CreatedAt = time.Date(2020, 11, 17, 15, 54, 33, 0, time.UTC)
			want := []*base.Comment{}
			if testcase.want != nil {
				testcase.want.Created, testcase.want.Updated = item.CreatedAt, item.CreatedAt
				want = append(want, testcase.want)
			}
			assert.Equal(t, want, convertTimelineItems([]gqlTimelineItem{item}))
		})
	}
}

func TestDropCloseOnMerge(t *testing.T) {
	for _, testcase := range []struct {
		name   string
		events []string
		want   []string
	}{
		{"unmerged close stays", []string{"close"}, []string{"close"}},
		{"last close after merge drops", []string{"close", "reopen", "merge_pull", "close"}, []string{"close", "reopen", "merge_pull"}},
		{"last close before merge drops", []string{"close", "merge_pull"}, []string{"merge_pull"}},
		{"close followed by reopen stays", []string{"close", "merge_pull", "reopen"}, []string{"close", "merge_pull", "reopen"}},
		{"merge without close stays", []string{"merge_pull"}, []string{"merge_pull"}},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			var events []*base.Comment
			for _, event := range testcase.events {
				events = append(events, &base.Comment{CommentType: event})
			}
			var got []string
			for _, event := range dropCloseOnMerge(events) {
				got = append(got, event.CommentType)
			}
			assert.Equal(t, testcase.want, got)
		})
	}
}
