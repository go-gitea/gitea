// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"sync"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
	notify_service "gitea.dev/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue_AddLabels(t *testing.T) {
	tests := []struct {
		issueID  int64
		labelIDs []int64
		doerID   int64
	}{
		{1, []int64{1, 2}, 2}, // non-pull-request
		{1, []int64{}, 2},     // non-pull-request, empty
		{2, []int64{1, 2}, 2}, // pull-request
		{2, []int64{}, 1},     // pull-request, empty
	}
	for _, test := range tests {
		assert.NoError(t, unittest.PrepareTestDatabase())
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: test.issueID})
		labels := make([]*issues_model.Label, len(test.labelIDs))
		for i, labelID := range test.labelIDs {
			labels[i] = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: labelID})
		}
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: test.doerID})
		assert.NoError(t, AddLabels(t.Context(), issue, doer, labels))
		for _, labelID := range test.labelIDs {
			unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: test.issueID, LabelID: labelID})
		}
	}
}

func TestIssue_AddLabel(t *testing.T) {
	tests := []struct {
		issueID int64
		labelID int64
		doerID  int64
	}{
		{1, 2, 2}, // non-pull-request, not-already-added label
		{1, 1, 2}, // non-pull-request, already-added label
		{2, 2, 2}, // pull-request, not-already-added label
		{2, 1, 2}, // pull-request, already-added label
	}
	for _, test := range tests {
		assert.NoError(t, unittest.PrepareTestDatabase())
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: test.issueID})
		label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: test.labelID})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: test.doerID})
		assert.NoError(t, AddLabel(t.Context(), issue, doer, label))
		unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: test.issueID, LabelID: test.labelID})
	}
}

type labelChangeRecorder struct {
	notify_service.NullNotifier
	calls          int
	added, removed []int64
}

func (r *labelChangeRecorder) IssueChangeLabels(_ context.Context, _ *user_model.User, _ *issues_model.Issue, added, removed []*issues_model.Label) {
	r.calls++
	r.added, r.removed = labelIDs(added), labelIDs(removed)
}

func labelIDs(labels []*issues_model.Label) (ids []int64) {
	for _, label := range labels {
		ids = append(ids, label.ID)
	}
	return ids
}

var registerLabelChangeRecorder = sync.OnceValue(func() *labelChangeRecorder {
	r := &labelChangeRecorder{}
	notify_service.RegisterNotifier(r)
	return r
})

func TestIssue_AddRemoveLabels(t *testing.T) {
	recorder := registerLabelChangeRecorder()
	tests := []struct {
		name                   string
		add, remove            []int64
		wantLabels             []int64
		wantAdded, wantRemoved []int64
	}{
		{"swap", []int64{2}, []int64{1}, []int64{2}, []int64{2}, []int64{1}},
		{"no change", []int64{1}, []int64{2}, []int64{1}, nil, nil},
		{"label of another owner", []int64{3}, nil, []int64{1}, nil, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
			doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
			add, err := issues_model.GetLabelsByIDs(t.Context(), test.add)
			require.NoError(t, err)
			remove, err := issues_model.GetLabelsByIDs(t.Context(), test.remove)
			require.NoError(t, err)
			*recorder = labelChangeRecorder{}

			require.NoError(t, AddRemoveLabels(t.Context(), issue, doer, add, remove))

			labels, err := issues_model.GetLabelsByIssueID(t.Context(), issue.ID)
			require.NoError(t, err)
			assert.Equal(t, test.wantLabels, labelIDs(labels))
			assert.Equal(t, util.Iif(test.wantAdded == nil, 0, 1), recorder.calls)
			assert.Equal(t, test.wantAdded, recorder.added)
			assert.Equal(t, test.wantRemoved, recorder.removed)
		})
	}
}
