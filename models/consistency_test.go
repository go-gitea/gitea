// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestDeleteOrphanedObjects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	countBefore, err := db.GetEngine(db.DefaultContext).Count(&PullRequest{})
	assert.NoError(t, err)

	_, err = db.GetEngine(db.DefaultContext).Insert(&PullRequest{IssueID: 1000}, &PullRequest{IssueID: 1001}, &PullRequest{IssueID: 1003})
	assert.NoError(t, err)

	orphaned, err := CountOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, orphaned)

	err = DeleteOrphanedObjects("pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)

	countAfter, err := db.GetEngine(db.DefaultContext).Count(&PullRequest{})
	assert.NoError(t, err)
	assert.EqualValues(t, countBefore, countAfter)
}

func TestNewMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := &issues_model.Milestone{
		RepoID:  1,
		Name:    "milestoneName",
		Content: "milestoneContent",
	}

	assert.NoError(t, issues_model.NewMilestone(milestone))
	unittest.AssertExistsAndLoadBean(t, milestone)
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})
}

func TestChangeMilestoneStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}).(*issues_model.Milestone)

	assert.NoError(t, issues_model.ChangeMilestoneStatus(milestone, true))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}, "is_closed=1")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})

	assert.NoError(t, issues_model.ChangeMilestoneStatus(milestone, false))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}, "is_closed=0")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})
}

func TestDeleteMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.NoError(t, issues_model.DeleteMilestoneByRepoID(1, 1))
	unittest.AssertNotExistsBean(t, &issues_model.Milestone{ID: 1})
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: 1})

	assert.NoError(t, issues_model.DeleteMilestoneByRepoID(unittest.NonexistentID, unittest.NonexistentID))
}

func TestUpdateMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}).(*issues_model.Milestone)
	milestone.Name = " newMilestoneName  "
	milestone.Content = "newMilestoneContent"
	assert.NoError(t, issues_model.UpdateMilestone(milestone, milestone.IsClosed))
	milestone = unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}).(*issues_model.Milestone)
	assert.EqualValues(t, "newMilestoneName", milestone.Name)
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})
}

func TestUpdateMilestoneCounters(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{MilestoneID: 1},
		"is_closed=0").(*Issue)

	issue.IsClosed = true
	issue.ClosedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})

	issue.IsClosed = false
	issue.ClosedUnix = 0
	_, err = db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})
}

func TestConsistencyUpdateAction(t *testing.T) {
	if !setting.Database.UseSQLite3 {
		t.Skip("Test is only for SQLite database.")
	}
	assert.NoError(t, unittest.PrepareTestDatabase())
	id := 8
	unittest.AssertExistsAndLoadBean(t, &Action{
		ID: int64(id),
	})
	_, err := db.GetEngine(db.DefaultContext).Exec(`UPDATE action SET created_unix = "" WHERE id = ?`, id)
	assert.NoError(t, err)
	actions := make([]*Action, 0, 1)
	//
	// XORM returns an error when created_unix is a string
	//
	err = db.GetEngine(db.DefaultContext).Where("id = ?", id).Find(&actions)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "type string to a int64: invalid syntax")
	}
	//
	// Get rid of incorrectly set created_unix
	//
	count, err := CountActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)
	count, err = FixActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = CountActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
	count, err = FixActionCreatedUnixString()
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)

	//
	// XORM must be happy now
	//
	assert.NoError(t, db.GetEngine(db.DefaultContext).Where("id = ?", id).Find(&actions))
	unittest.CheckConsistencyFor(t, &Action{})
}
