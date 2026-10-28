// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/references"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestCombineXRefComments(t *testing.T) {
	none, closes, neutered := references.XRefActionNone, references.XRefActionCloses, references.XRefActionNeutered
	xref := func(id, refIssueID, refCommentID int64, action references.XRefAction) *issues_model.Comment {
		return &issues_model.Comment{ID: id, Type: issues_model.CommentTypeIssueRef, RefIssueID: refIssueID, RefCommentID: refCommentID, RefAction: action}
	}
	issue := issues_model.Issue{Comments: issues_model.CommentList{
		xref(1, 10, 1, neutered),
		xref(2, 11, 2, none),
		xref(3, 10, 3, none),
		xref(4, 11, 4, closes),
		xref(5, 11, 5, neutered),
		xref(6, 0, 0, none),
		xref(7, 0, 0, none),
	}}
	combineXRefComments(&issue)
	assert.Equal(t, issues_model.CommentList{
		xref(1, 10, 3, none),
		xref(2, 11, 2, closes),
		xref(6, 0, 0, none),
		xref(7, 0, 0, none),
	}, issue.Comments)
}

func TestCombineLabelComments(t *testing.T) {
	labelComment := func(posterID int64, content, labelName string, createdUnix timeutil.TimeStamp, addedLabels, removedLabels []*issues_model.Label) *issues_model.Comment {
		return &issues_model.Comment{
			Type:          issues_model.CommentTypeLabel,
			PosterID:      posterID,
			Content:       content,
			Label:         &issues_model.Label{Name: labelName},
			CreatedUnix:   createdUnix,
			AddedLabels:   addedLabels,
			RemovedLabels: removedLabels,
		}
	}
	comment := func(posterID int64, content string) *issues_model.Comment {
		return &issues_model.Comment{Type: issues_model.CommentTypeComment, PosterID: posterID, Content: content, CreatedUnix: 0}
	}
	kases := []struct {
		name           string
		beforeCombined []*issues_model.Comment
		afterCombined  []*issues_model.Comment
	}{
		{
			name: "kase 1",
			beforeCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, nil, nil),
				labelComment(1, "", "kind/bug", 0, nil, nil),
				comment(1, "test"),
			},
			afterCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, []*issues_model.Label{}, nil),
				comment(1, "test"),
			},
		},
		{
			name: "kase 2",
			beforeCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, nil, nil),
				labelComment(1, "", "kind/bug", 70, nil, nil),
				comment(1, "test"),
			},
			afterCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, []*issues_model.Label{{Name: "kind/bug"}}, nil),
				labelComment(1, "", "kind/bug", 70, nil, []*issues_model.Label{{Name: "kind/bug"}}),
				comment(1, "test"),
			},
		},
		{
			name: "kase 3",
			beforeCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, nil, nil),
				labelComment(2, "", "kind/bug", 0, nil, nil),
				comment(1, "test"),
			},
			afterCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, []*issues_model.Label{{Name: "kind/bug"}}, nil),
				labelComment(2, "", "kind/bug", 0, nil, []*issues_model.Label{{Name: "kind/bug"}}),
				comment(1, "test"),
			},
		},
		{
			name: "migrated events of different original authors",
			beforeCombined: []*issues_model.Comment{
				{Type: issues_model.CommentTypeComment, PosterID: 1},
				{Type: issues_model.CommentTypeLabel, PosterID: 1, OriginalAuthor: "alice", Content: "1", Label: &issues_model.Label{Name: "kind/bug"}},
				{Type: issues_model.CommentTypeLabel, PosterID: 1, OriginalAuthor: "bob", Content: "1", Label: &issues_model.Label{Name: "kind/ui"}},
			},
			afterCombined: []*issues_model.Comment{
				{Type: issues_model.CommentTypeComment, PosterID: 1},
				{Type: issues_model.CommentTypeLabel, PosterID: 1, OriginalAuthor: "alice", Content: "1", Label: &issues_model.Label{Name: "kind/bug"}, AddedLabels: []*issues_model.Label{{Name: "kind/bug"}}},
				{Type: issues_model.CommentTypeLabel, PosterID: 1, OriginalAuthor: "bob", Content: "1", Label: &issues_model.Label{Name: "kind/ui"}, AddedLabels: []*issues_model.Label{{Name: "kind/ui"}}},
			},
		},
		{
			name: "kase 4",
			beforeCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, nil, nil),
				labelComment(1, "1", "kind/backport", 10, nil, nil),
			},
			afterCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 10, []*issues_model.Label{{Name: "kind/bug"}, {Name: "kind/backport"}}, nil),
			},
		},
		{
			name: "kase 5",
			beforeCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, nil, nil),
				comment(2, "testtest"),
				labelComment(1, "", "kind/bug", 0, nil, nil),
			},
			afterCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, []*issues_model.Label{{Name: "kind/bug"}}, nil),
				comment(2, "testtest"),
				labelComment(1, "", "kind/bug", 0, nil, []*issues_model.Label{{Name: "kind/bug"}}),
			},
		},
		{
			name: "kase 6",
			beforeCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, nil, nil),
				labelComment(1, "1", "reviewed/confirmed", 0, nil, nil),
				labelComment(1, "", "kind/bug", 0, nil, nil),
				labelComment(1, "1", "kind/feature", 0, nil, nil),
			},
			afterCombined: []*issues_model.Comment{
				labelComment(1, "1", "kind/bug", 0, []*issues_model.Label{{Name: "reviewed/confirmed"}, {Name: "kind/feature"}}, nil),
			},
		},
		{
			name: "project workflow",
			beforeCombined: []*issues_model.Comment{
				{Type: issues_model.CommentTypeLabel, PosterID: 1, Content: "1", Label: &issues_model.Label{Name: "kind/bug"}},
				{Type: issues_model.CommentTypeLabel, PosterID: 1, Content: "1", Label: &issues_model.Label{Name: "status/done"}, CommentMetaData: &issues_model.CommentMetaData{SpecialDoerName: issues_model.SpecialDoerNameProjectWorkflow}},
			},
			afterCombined: []*issues_model.Comment{
				{Type: issues_model.CommentTypeLabel, PosterID: 1, Content: "1", Label: &issues_model.Label{Name: "kind/bug"}, AddedLabels: []*issues_model.Label{{Name: "kind/bug"}}},
				{Type: issues_model.CommentTypeLabel, PosterID: 1, Content: "1", Label: &issues_model.Label{Name: "status/done"}, AddedLabels: []*issues_model.Label{{Name: "status/done"}}, CommentMetaData: &issues_model.CommentMetaData{SpecialDoerName: issues_model.SpecialDoerNameProjectWorkflow}},
			},
		},
	}

	for _, kase := range kases {
		t.Run(kase.name, func(t *testing.T) {
			issue := issues_model.Issue{
				Comments: kase.beforeCombined,
			}
			combineLabelComments(&issue)
			assert.EqualValues(t, kase.afterCombined, issue.Comments)
		})
	}
}
