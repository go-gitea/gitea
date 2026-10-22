// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
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

func (r *labelChangeRecorder) IssueChangeLabels(_ctx context.Context, _doer *user_model.User, _issue *issues_model.Issue, added, removed []*issues_model.Label) {
	r.calls++
	r.added, r.removed = labelIDs(added), labelIDs(removed)
}

func labelIDs(labels []*issues_model.Label) (ids []int64) {
	for _, label := range labels {
		ids = append(ids, label.ID)
	}
	return ids
}

func TestIssue_AddRemoveLabels(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	labels := func(ids ...int64) []*issues_model.Label {
		labels, err := issues_model.GetLabelsByIDs(ctx, ids)
		require.NoError(t, err)
		return labels
	}
	*labelChanges = labelChangeRecorder{}

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	require.NoError(t, AddRemoveLabels(ctx, issue, doer, labels(1, 3), labels(2)))
	assert.Equal(t, []int64{1}, labelIDs(issue.Labels))
	assert.Equal(t, labelChangeRecorder{}, *labelChanges)

	require.NoError(t, AddRemoveLabels(ctx, issue, doer, labels(2), labels(1)))
	assert.Equal(t, []int64{2}, labelIDs(issue.Labels))
	assert.Equal(t, labelChangeRecorder{calls: 1, added: []int64{2}, removed: []int64{1}}, *labelChanges)

	exclusive := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 18})
	require.NoError(t, AddRemoveLabels(ctx, exclusive, doer, labels(7), nil))
	*labelChanges = labelChangeRecorder{}
	require.NoError(t, AddRemoveLabels(ctx, exclusive, doer, labels(8), nil))
	assert.Equal(t, []int64{8}, labelIDs(exclusive.Labels))
	assert.Equal(t, labelChangeRecorder{calls: 1, added: []int64{8}, removed: []int64{7}}, *labelChanges)
}
