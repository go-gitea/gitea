// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"sort"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestMilestone_State(t *testing.T) {
	assert.Equal(t, api.StateOpen, (&issues_model.Milestone{IsClosed: false}).State())
	assert.Equal(t, api.StateClosed, (&issues_model.Milestone{IsClosed: true}).State())
}

func TestMilestoneQueries(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	t.Run("GetMilestoneByRepoID", func(t *testing.T) {
		milestone, err := issues_model.GetMilestoneByRepoID(t.Context(), 1, 1)
		assert.NoError(t, err)
		assert.EqualValues(t, 1, milestone.ID)
		assert.EqualValues(t, 1, milestone.RepoID)

		_, err = issues_model.GetMilestoneByRepoID(t.Context(), unittest.NonexistentID, unittest.NonexistentID)
		assert.True(t, issues_model.IsErrMilestoneNotExist(err))
	})
	t.Run("GetMilestonesByRepoID", func(t *testing.T) {
		test := func(repoID int64, state api.StateType) {
			var isClosed optional.Option[bool]
			switch state {
			case api.StateClosed, api.StateOpen:
				isClosed = optional.Some(state == api.StateClosed)
			}
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
			milestones, err := db.Find[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
				RepoID:   repo.ID,
				IsClosed: isClosed,
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
				assert.Equal(t, repoID, milestone.RepoID)
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

		milestones, err := db.Find[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
			RepoID:   unittest.NonexistentID,
			IsClosed: optional.Some(false),
		})
		assert.NoError(t, err)
		assert.Empty(t, milestones)
	})
	t.Run("GetMilestones", func(t *testing.T) {
		test := func(t *testing.T, options issues_model.FindMilestoneOptions, numOpen, numClosed int) {
			testSort := func(sortType string, sortCond func(*issues_model.Milestone) int) {
				for _, page := range []int{0, 1} {
					for _, isClosed := range []bool{false, true} {
						options.ListOptions = db.ListOptions{
							Page:     page,
							PageSize: setting.UI.IssuePagingNum,
						}
						options.IsClosed = optional.Some(isClosed)
						options.SortType = sortType
						milestones, err := db.Find[issues_model.Milestone](t.Context(), options)
						assert.NoError(t, err)
						expectedCount := numOpen
						if isClosed {
							expectedCount = numClosed
						}
						assert.Len(t, milestones, expectedCount)
						values := make([]int, len(milestones))
						for i, milestone := range milestones {
							values[i] = sortCond(milestone)
						}
						assert.True(t, sort.IntsAreSorted(values))
					}
				}
			}
			testSort("furthestduedate", func(milestone *issues_model.Milestone) int {
				return -int(milestone.DeadlineUnix)
			})
			testSort("leastcomplete", func(milestone *issues_model.Milestone) int {
				return milestone.Completeness
			})
			testSort("mostcomplete", func(milestone *issues_model.Milestone) int {
				return -milestone.Completeness
			})
			testSort("leastissues", func(milestone *issues_model.Milestone) int {
				return milestone.NumIssues
			})
			testSort("mostissues", func(milestone *issues_model.Milestone) int {
				return -milestone.NumIssues
			})
			testSort("soonestduedate", func(milestone *issues_model.Milestone) int {
				return int(milestone.DeadlineUnix)
			})
		}
		t.Run("repo", func(t *testing.T) {
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
			test(t, issues_model.FindMilestoneOptions{RepoID: repo.ID}, repo.NumMilestones-repo.NumClosedMilestones, repo.NumClosedMilestones)
		})
		t.Run("repos", func(t *testing.T) {
			repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
			repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
			test(t, issues_model.FindMilestoneOptions{RepoIDs: []int64{repo1.ID, repo2.ID}},
				repo1.NumOpenMilestones+repo2.NumOpenMilestones, repo1.NumClosedMilestones+repo2.NumClosedMilestones)
		})
	})
	t.Run("CountRepoMilestones", func(t *testing.T) {
		test := func(repoID int64) {
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
			count, err := db.Count[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
				RepoID: repoID,
			})
			assert.NoError(t, err)
			assert.EqualValues(t, repo.NumMilestones, count)
		}
		test(1)
		test(2)
		test(3)

		count, err := db.Count[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
			RepoID: unittest.NonexistentID,
		})
		assert.NoError(t, err)
		assert.EqualValues(t, 0, count)
	})
	t.Run("CountRepoClosedMilestones", func(t *testing.T) {
		test := func(repoID int64) {
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
			count, err := db.Count[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
				RepoID:   repoID,
				IsClosed: optional.Some(true),
			})
			assert.NoError(t, err)
			assert.EqualValues(t, repo.NumClosedMilestones, count)
		}
		test(1)
		test(2)
		test(3)

		count, err := db.Count[issues_model.Milestone](t.Context(), issues_model.FindMilestoneOptions{
			RepoID:   unittest.NonexistentID,
			IsClosed: optional.Some(true),
		})
		assert.NoError(t, err)
		assert.EqualValues(t, 0, count)
	})
	t.Run("CountMilestonesByRepoIDs", func(t *testing.T) {
		milestonesCount := func(repoID int64) (int, int) {
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repoID})
			return repo.NumOpenMilestones, repo.NumClosedMilestones
		}
		repo1OpenCount, repo1ClosedCount := milestonesCount(1)
		repo2OpenCount, repo2ClosedCount := milestonesCount(2)

		openCounts, err := issues_model.CountMilestonesMap(t.Context(), issues_model.FindMilestoneOptions{
			RepoIDs:  []int64{1, 2},
			IsClosed: optional.Some(false),
		})
		assert.NoError(t, err)
		assert.EqualValues(t, repo1OpenCount, openCounts[1])
		assert.EqualValues(t, repo2OpenCount, openCounts[2])

		closedCounts, err := issues_model.CountMilestonesMap(t.Context(),
			issues_model.FindMilestoneOptions{
				RepoIDs:  []int64{1, 2},
				IsClosed: optional.Some(true),
			})
		assert.NoError(t, err)
		assert.EqualValues(t, repo1ClosedCount, closedCounts[1])
		assert.EqualValues(t, repo2ClosedCount, closedCounts[2])
	})
}

func TestNewMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := &issues_model.Milestone{
		RepoID:  1,
		Name:    "milestoneName",
		Content: "milestoneContent",
	}

	assert.NoError(t, issues_model.NewMilestone(t.Context(), milestone))
	unittest.AssertExistsAndLoadBean(t, milestone)
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})
}

func TestChangeMilestoneStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})

	assert.NoError(t, issues_model.ChangeMilestoneStatus(t.Context(), milestone, true))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}, "is_closed=1")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})

	assert.NoError(t, issues_model.ChangeMilestoneStatus(t.Context(), milestone, false))
	unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}, "is_closed=0")
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: milestone.RepoID}, &issues_model.Milestone{})
}

func TestDeleteMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.NoError(t, issues_model.DeleteMilestoneByRepoID(t.Context(), 1, 1))
	unittest.AssertNotExistsBean(t, &issues_model.Milestone{ID: 1})
	unittest.CheckConsistencyFor(t, &repo_model.Repository{ID: 1})

	assert.NoError(t, issues_model.DeleteMilestoneByRepoID(t.Context(), unittest.NonexistentID, unittest.NonexistentID))
}

func TestUpdateMilestone(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})
	milestone.Name = " newMilestoneName  "
	milestone.Content = "newMilestoneContent"
	assert.NoError(t, issues_model.UpdateMilestone(t.Context(), milestone, milestone.IsClosed))
	milestone = unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})
	assert.Equal(t, "newMilestoneName", milestone.Name)
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})
}

func TestUpdateMilestoneCounters(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{MilestoneID: 1},
		"is_closed=0")

	issue.IsClosed = true
	issue.ClosedUnix = timeutil.TimeStampNow()
	_, err := db.GetEngine(t.Context()).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(t.Context(), issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})

	issue.IsClosed = false
	issue.ClosedUnix = 0
	_, err = db.GetEngine(t.Context()).ID(issue.ID).Cols("is_closed", "closed_unix").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, issues_model.UpdateMilestoneCounters(t.Context(), issue.MilestoneID))
	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})
}

func TestMigrate_InsertMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame})
	name := "milestonetest1"
	ms := &issues_model.Milestone{
		RepoID: repo.ID,
		Name:   name,
	}
	err := issues_model.InsertMilestones(t.Context(), ms)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, ms)
	repoModified := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	assert.Equal(t, repo.NumMilestones+1, repoModified.NumMilestones)

	unittest.CheckConsistencyFor(t, &issues_model.Milestone{})
}
