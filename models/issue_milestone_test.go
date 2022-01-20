// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func TestMilestone_State(t *testing.T) {
	assert.Equal(t, api.StateOpen, (&Milestone{IsClosed: false}).State())
	assert.Equal(t, api.StateClosed, (&Milestone{IsClosed: true}).State())
}

func TestNewMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := &Milestone{
		RepoID:  1,
		Name:    "milestoneName",
		Content: "milestoneContent",
	}

	assert.NoError(t, NewMilestone(milestone))
	unittest.AssertExistsAndLoadBean(t, milestone)
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &Milestone{})
}

func TestGetMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	milestone, err := GetMilestoneByRepoID(1, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, milestone.ID)
	assert.EqualValues(t, 1, milestone.RepoID)

	_, err = GetMilestoneByRepoID(unittest.NonexistentID, unittest.NonexistentID)
	assert.True(t, IsErrMilestoneNotExist(err))
}

func TestGetMilestonesByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64, state api.StateType) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		milestones, _, err := GetMilestones(GetMilestonesOption{
			RepoID: repo.ID,
			State:  state,
		})
		assert.NoError(t, err)

		var n int

		switch state {
		case api.StateClosed:
			n = repo.NumClosedMilestones

		case api.StateAll:
			n = repo.NumMilestones

		case api.StateOpen:
			fallthrough

		default:
			n = repo.NumOpenMilestones
		}

		assert.Len(t, milestones, n)
		for _, milestone := range milestones {
			assert.EqualValues(t, repoID, milestone.RepoID)
		}
	}
	test(1, api.StateOpen)
	test(1, api.StateAll)
	test(1, api.StateClosed)
	test(2, api.StateOpen)
	test(2, api.StateAll)
	test(2, api.StateClosed)
	test(3, api.StateOpen)
	test(3, api.StateClosed)
	test(3, api.StateAll)

	milestones, _, err := GetMilestones(GetMilestonesOption{
		RepoID: unittest.NonexistentID,
		State:  api.StateOpen,
	})
	assert.NoError(t, err)
	assert.Len(t, milestones, 0)
}

func TestGetMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	test := func(sortType string, sortCond func(*Milestone) int) {
		for _, page := range []int{0, 1} {
			milestones, _, err := GetMilestones(GetMilestonesOption{
				ListOptions: db.ListOptions{
					Page:     page,
					PageSize: setting.UI.IssuePagingNum,
				},
				RepoID:   repo.ID,
				State:    api.StateOpen,
				SortType: sortType,
			})
			assert.NoError(t, err)
			assert.Len(t, milestones, repo.NumMilestones-repo.NumClosedMilestones)
			values := make([]int, len(milestones))
			for i, milestone := range milestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))

			milestones, _, err = GetMilestones(GetMilestonesOption{
				ListOptions: db.ListOptions{
					Page:     page,
					PageSize: setting.UI.IssuePagingNum,
				},
				RepoID:   repo.ID,
				State:    api.StateClosed,
				Name:     "",
				SortType: sortType,
			})
			assert.NoError(t, err)
			assert.Len(t, milestones, repo.NumClosedMilestones)
			values = make([]int, len(milestones))
			for i, milestone := range milestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))
		}
	}
	test("furthestduedate", func(milestone *Milestone) int {
		return -int(milestone.DeadlineUnix)
	})
	test("leastcomplete", func(milestone *Milestone) int {
		return milestone.Completeness
	})
	test("mostcomplete", func(milestone *Milestone) int {
		return -milestone.Completeness
	})
	test("leastissues", func(milestone *Milestone) int {
		return milestone.NumIssues
	})
	test("mostissues", func(milestone *Milestone) int {
		return -milestone.NumIssues
	})
	test("soonestduedate", func(milestone *Milestone) int {
		return int(milestone.DeadlineUnix)
	})
}

func TestUpdateMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	milestone := unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone)
	milestone.Name = " newMilestoneName  "
	milestone.Content = "newMilestoneContent"
	assert.NoError(t, UpdateMilestone(milestone, milestone.IsClosed))
	milestone = unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone)
	assert.EqualValues(t, "newMilestoneName", milestone.Name)
	unittest.CheckConsistencyFor(t, &Milestone{})
}

func TestCountRepoMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		count, err := countRepoMilestones(db.GetEngine(db.DefaultContext), repoID)
		assert.NoError(t, err)
		assert.EqualValues(t, repo.NumMilestones, count)
	}
	test(1)
	test(2)
	test(3)

	count, err := countRepoMilestones(db.GetEngine(db.DefaultContext), unittest.NonexistentID)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestCountRepoClosedMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		count, err := CountRepoClosedMilestones(repoID)
		assert.NoError(t, err)
		assert.EqualValues(t, repo.NumClosedMilestones, count)
	}
	test(1)
	test(2)
	test(3)

	count, err := CountRepoClosedMilestones(unittest.NonexistentID)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

func TestChangeMilestoneStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone)

	assert.NoError(t, ChangeMilestoneStatus(milestone, true))
	unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}, "is_closed=1")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &Milestone{})

	assert.NoError(t, ChangeMilestoneStatus(milestone, false))
	unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}, "is_closed=0")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &Milestone{})
}

func TestUpdateMilestoneCounters(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{MilestoneID: 1},
		"is_closed=0").(*Issue)

	issue.IsClosed = true
	issue.ClosedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, updateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &Milestone{})

	issue.IsClosed = false
	issue.ClosedUnix = 0
	_, err = db.GetEngine(db.DefaultContext).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, updateMilestoneCounters(db.DefaultContext, issue.MilestoneID))
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
	assert.NoError(t, ChangeMilestoneAssign(issue, doer, oldMilestoneID))
	unittest.AssertExistsAndLoadBean(t, &Comment{
		IssueID:        issue.ID,
		Type:           CommentTypeMilestone,
		MilestoneID:    issue.MilestoneID,
		OldMilestoneID: oldMilestoneID,
	})
	unittest.CheckConsistencyFor(t, &Milestone{}, &Issue{})
}

func TestDeleteMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.NoError(t, DeleteMilestoneByRepoID(1, 1))
	unittest.AssertNotExistsBean(t, &Milestone{ID: 1})
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: 1})

	assert.NoError(t, DeleteMilestoneByRepoID(unittest.NonexistentID, unittest.NonexistentID))
}

func TestMilestoneList_LoadTotalTrackedTimes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	miles := MilestoneList{
		unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone),
	}

	assert.NoError(t, miles.LoadTotalTrackedTimes())

	assert.Equal(t, int64(3682), miles[0].TotalTrackedTime)
}

func TestCountMilestonesByRepoIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestonesCount := func(repoID int64) (int, int) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		return repo.NumOpenMilestones, repo.NumClosedMilestones
	}
	repo1OpenCount, repo1ClosedCount := milestonesCount(1)
	repo2OpenCount, repo2ClosedCount := milestonesCount(2)

	openCounts, err := CountMilestonesByRepoCond(builder.In("repo_id", []int64{1, 2}), false)
	assert.NoError(t, err)
	assert.EqualValues(t, repo1OpenCount, openCounts[1])
	assert.EqualValues(t, repo2OpenCount, openCounts[2])

	closedCounts, err := CountMilestonesByRepoCond(builder.In("repo_id", []int64{1, 2}), true)
	assert.NoError(t, err)
	assert.EqualValues(t, repo1ClosedCount, closedCounts[1])
	assert.EqualValues(t, repo2ClosedCount, closedCounts[2])
}

func TestGetMilestonesByRepoIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)
	test := func(sortType string, sortCond func(*Milestone) int) {
		for _, page := range []int{0, 1} {
			openMilestones, err := GetMilestonesByRepoIDs([]int64{repo1.ID, repo2.ID}, page, false, sortType)
			assert.NoError(t, err)
			assert.Len(t, openMilestones, repo1.NumOpenMilestones+repo2.NumOpenMilestones)
			values := make([]int, len(openMilestones))
			for i, milestone := range openMilestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))

			closedMilestones, err := GetMilestonesByRepoIDs([]int64{repo1.ID, repo2.ID}, page, true, sortType)
			assert.NoError(t, err)
			assert.Len(t, closedMilestones, repo1.NumClosedMilestones+repo2.NumClosedMilestones)
			values = make([]int, len(closedMilestones))
			for i, milestone := range closedMilestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))
		}
	}
	test("furthestduedate", func(milestone *Milestone) int {
		return -int(milestone.DeadlineUnix)
	})
	test("leastcomplete", func(milestone *Milestone) int {
		return milestone.Completeness
	})
	test("mostcomplete", func(milestone *Milestone) int {
		return -milestone.Completeness
	})
	test("leastissues", func(milestone *Milestone) int {
		return milestone.NumIssues
	})
	test("mostissues", func(milestone *Milestone) int {
		return -milestone.NumIssues
	})
	test("soonestduedate", func(milestone *Milestone) int {
		return int(milestone.DeadlineUnix)
	})
}

func TestLoadTotalTrackedTime(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone)

	assert.NoError(t, milestone.LoadTotalTrackedTime())

	assert.Equal(t, int64(3682), milestone.TotalTrackedTime)
}

func TestGetMilestonesStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	test := func(repoID int64) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID}).(*repo_model.Repository)
		stats, err := GetMilestonesStatsByRepoCond(builder.And(builder.Eq{"repo_id": repoID}))
		assert.NoError(t, err)
		assert.EqualValues(t, repo.NumMilestones-repo.NumClosedMilestones, stats.OpenCount)
		assert.EqualValues(t, repo.NumClosedMilestones, stats.ClosedCount)
	}
	test(1)
	test(2)
	test(3)

	stats, err := GetMilestonesStatsByRepoCond(builder.And(builder.Eq{"repo_id": unittest.NonexistentID}))
	assert.NoError(t, err)
	assert.EqualValues(t, 0, stats.OpenCount)
	assert.EqualValues(t, 0, stats.ClosedCount)

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}).(*repo_model.Repository)

	milestoneStats, err := GetMilestonesStatsByRepoCond(builder.In("repo_id", []int64{repo1.ID, repo2.ID}))
	assert.NoError(t, err)
	assert.EqualValues(t, repo1.NumOpenMilestones+repo2.NumOpenMilestones, milestoneStats.OpenCount)
	assert.EqualValues(t, repo1.NumClosedMilestones+repo2.NumClosedMilestones, milestoneStats.ClosedCount)
}
