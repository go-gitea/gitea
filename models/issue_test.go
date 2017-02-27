<<<<<<< c67fc79f985bd7a2c077ec18cc5538ede3242489
// Copyright 2017 The Gitea Authors. All rights reserved.
=======
// Copyright 2017 The Gogs Authors. All rights reserved.
>>>>>>> fix: Add unit testing.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIssueReplaceLabels(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(issueID int64, labelIDs []int64) {
		issue := AssertExistsAndLoadBean(t, &Issue{ID: issueID}).(*Issue)
		repo := AssertExistsAndLoadBean(t, &Repository{ID: issue.RepoID}).(*Repository)
		doer := AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)

		labels := make([]*Label, len(labelIDs))
		for i, labelID := range labelIDs {
			labels[i] = AssertExistsAndLoadBean(t, &Label{ID: labelID, RepoID: repo.ID}).(*Label)
		}
		assert.NoError(t, issue.ReplaceLabels(labels, doer))
		AssertCount(t, &IssueLabel{IssueID: issueID}, len(labelIDs))
		for _, labelID := range labelIDs {
			AssertExistsAndLoadBean(t, &IssueLabel{IssueID: issueID, LabelID: labelID})
		}
	}

	testSuccess(1, []int64{2})
	testSuccess(1, []int64{1, 2})
	testSuccess(1, []int64{})
}

func TestIssueAPIURL(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	err := issue.LoadAttributes()

	assert.NoError(t, err)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/issues/1", issue.APIURL())
}
