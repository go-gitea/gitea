// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
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
		assert.NoError(t, AddLabels(db.DefaultContext, issue, doer, labels))
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
		assert.NoError(t, AddLabel(db.DefaultContext, issue, doer, label))
		unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: test.issueID, LabelID: test.labelID})
	}
}
