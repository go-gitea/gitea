// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	"github.com/stretchr/testify/assert"
)

func TestUpdateMilestoneCounters(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{MilestoneID: 1},
		"is_closed=0").(*Issue)

	issue.IsClosed = true
	issue.ClosedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &Milestone{})

	issue.IsClosed = false
	issue.ClosedUnix = 0
	_, err = db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &Milestone{})
}

func TestChangeMilestoneAssign(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{RepoID: 1}).(*Issue)
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	assert.NotNil(t, issue)
	assert.NotNil(t, doer)

	oldMilestoneID := issue.MilestoneID
	issue.MilestoneID = 2
	assert.NoError(t, issues_model.ChangeMilestoneAssign(issue, doer, oldMilestoneID))
	unittest.AssertExistsAndLoadBean(t, &Comment{
		IssueID:        issue.ID,
		Type:           CommentTypeMilestone,
		MilestoneID:    issue.MilestoneID,
		OldMilestoneID: oldMilestoneID,
	})
	unittest.CheckConsistencyFor(t, &Milestone{}, &Issue{})
}
