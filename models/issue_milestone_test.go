// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"
	"testing"
	"time"

	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestMilestone_State(t *testing.T) {
	assert.Equal(t, api.StateOpen, (&Milestone{IsClosed: false}).State())
	assert.Equal(t, api.StateClosed, (&Milestone{IsClosed: true}).State())
}

func TestMilestone_APIFormat(t *testing.T) {
	milestone := &Milestone{
		ID:              3,
		RepoID:          4,
		Name:            "milestoneName",
		Content:         "milestoneContent",
		IsClosed:        false,
		NumOpenIssues:   5,
		NumClosedIssues: 6,
		Deadline:        time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, api.Milestone{
		ID:           milestone.ID,
		State:        api.StateOpen,
		Title:        milestone.Name,
		Description:  milestone.Content,
		OpenIssues:   milestone.NumOpenIssues,
		ClosedIssues: milestone.NumClosedIssues,
		Deadline:     &milestone.Deadline,
	}, *milestone.APIFormat())
}

func TestNewMilestone(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	milestone := &Milestone{
		RepoID:  1,
		Name:    "milestoneName",
		Content: "milestoneContent",
	}

	assert.NoError(t, NewMilestone(milestone))
	AssertExistsAndLoadBean(t, milestone)
	CheckConsistencyFor(t, &Repository{ID: milestone.RepoID}, &Milestone{})
}

func TestGetMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	milestone, err := GetMilestoneByRepoID(1, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, milestone.ID)
	assert.EqualValues(t, 1, milestone.RepoID)

	_, err = GetMilestoneByRepoID(NonexistentID, NonexistentID)
	assert.True(t, IsErrMilestoneNotExist(err))
}

func TestGetMilestonesByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(repoID int64) {
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		milestones, err := GetMilestonesByRepoID(repo.ID)
		assert.NoError(t, err)
		assert.Len(t, milestones, repo.NumMilestones)
		for _, milestone := range milestones {
			assert.EqualValues(t, repoID, milestone.RepoID)
		}
	}
	test(1)
	test(2)
	test(3)

	milestones, err := GetMilestonesByRepoID(NonexistentID)
	assert.NoError(t, err)
	assert.Len(t, milestones, 0)
}

func TestGetMilestones(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	test := func(sortType string, sortCond func(*Milestone) int) {
		for _, page := range []int{0, 1} {
			milestones, err := GetMilestones(repo.ID, page, false, sortType)
			assert.NoError(t, err)
			assert.Len(t, milestones, repo.NumMilestones-repo.NumClosedMilestones)
			values := make([]int, len(milestones))
			for i, milestone := range milestones {
				values[i] = sortCond(milestone)
			}
			assert.True(t, sort.IntsAreSorted(values))

			milestones, err = GetMilestones(repo.ID, page, true, sortType)
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
	assert.NoError(t, PrepareTestDatabase())

	milestone := AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone)
	milestone.Name = "newMilestoneName"
	milestone.Content = "newMilestoneContent"
	assert.NoError(t, UpdateMilestone(milestone))
	AssertExistsAndLoadBean(t, milestone)
	CheckConsistencyFor(t, &Milestone{})
}

func TestCountRepoMilestones(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(repoID int64) {
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		assert.EqualValues(t, repo.NumMilestones, countRepoMilestones(x, repoID))
	}
	test(1)
	test(2)
	test(3)
	assert.EqualValues(t, 0, countRepoMilestones(x, NonexistentID))
}

func TestCountRepoClosedMilestones(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(repoID int64) {
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		assert.EqualValues(t, repo.NumClosedMilestones, CountRepoClosedMilestones(repoID))
	}
	test(1)
	test(2)
	test(3)
	assert.EqualValues(t, 0, countRepoMilestones(x, NonexistentID))
}

func TestMilestoneStats(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	test := func(repoID int64) {
		repo := AssertExistsAndLoadBean(t, &Repository{ID: repoID}).(*Repository)
		open, closed := MilestoneStats(repoID)
		assert.EqualValues(t, repo.NumMilestones-repo.NumClosedMilestones, open)
		assert.EqualValues(t, repo.NumClosedMilestones, closed)
	}
	test(1)
	test(2)
	test(3)

	open, closed := MilestoneStats(NonexistentID)
	assert.EqualValues(t, 0, open)
	assert.EqualValues(t, 0, closed)
}

func TestChangeMilestoneStatus(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	milestone := AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone)

	assert.NoError(t, ChangeMilestoneStatus(milestone, true))
	AssertExistsAndLoadBean(t, &Milestone{ID: 1}, "is_closed=1")
	CheckConsistencyFor(t, &Repository{ID: milestone.RepoID}, &Milestone{})

	assert.NoError(t, ChangeMilestoneStatus(milestone, false))
	AssertExistsAndLoadBean(t, &Milestone{ID: 1}, "is_closed=0")
	CheckConsistencyFor(t, &Repository{ID: milestone.RepoID}, &Milestone{})
}

func TestChangeMilestoneIssueStats(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{MilestoneID: 1},
		"is_closed=0").(*Issue)

	issue.IsClosed = true
	_, err := x.Cols("is_closed").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, changeMilestoneIssueStats(x.NewSession(), issue))
	CheckConsistencyFor(t, &Milestone{})

	issue.IsClosed = false
	_, err = x.Cols("is_closed").Update(issue)
	assert.NoError(t, err)
	assert.NoError(t, changeMilestoneIssueStats(x.NewSession(), issue))
	CheckConsistencyFor(t, &Milestone{})
}

func TestChangeMilestoneAssign(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{RepoID: 1}).(*Issue)
	doer := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	oldMilestoneID := issue.MilestoneID
	issue.MilestoneID = 2
	assert.NoError(t, ChangeMilestoneAssign(issue, doer, oldMilestoneID))
	AssertExistsAndLoadBean(t, &Comment{
		IssueID:        issue.ID,
		Type:           CommentTypeMilestone,
		MilestoneID:    issue.MilestoneID,
		OldMilestoneID: oldMilestoneID,
	})
	CheckConsistencyFor(t, &Milestone{}, &Issue{})
}

func TestDeleteMilestoneByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, DeleteMilestoneByRepoID(1, 1))
	AssertNotExistsBean(t, &Milestone{ID: 1})
	CheckConsistencyFor(t, &Repository{ID: 1})

	assert.NoError(t, DeleteMilestoneByRepoID(NonexistentID, NonexistentID))
}
