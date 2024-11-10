// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"cmp"
	"slices"
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestIssueList_LoadRepositories(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}

	repos, err := issueList.LoadRepositories(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, repos, 2)
	for _, issue := range issueList {
		assert.EqualValues(t, issue.RepoID, issue.Repo.ID)
	}
}

func TestIssueList_LoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Service.EnableTimetracking = true
	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}

	assert.NoError(t, issueList.LoadAttributes(db.DefaultContext))
	for _, issue := range issueList {
		assert.EqualValues(t, issue.RepoID, issue.Repo.ID)
		for _, label := range issue.Labels {
			assert.EqualValues(t, issue.RepoID, label.RepoID)
			unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
		}
		if issue.PosterID > 0 {
			assert.EqualValues(t, issue.PosterID, issue.Poster.ID)
		}
		if issue.AssigneeID > 0 {
			assert.EqualValues(t, issue.AssigneeID, issue.Assignee.ID)
		}
		if issue.MilestoneID > 0 {
			assert.EqualValues(t, issue.MilestoneID, issue.Milestone.ID)
		}
		if issue.IsPull {
			assert.EqualValues(t, issue.ID, issue.PullRequest.IssueID)
		}
		for _, attachment := range issue.Attachments {
			assert.EqualValues(t, issue.ID, attachment.IssueID)
		}
		for _, comment := range issue.Comments {
			assert.EqualValues(t, issue.ID, comment.IssueID)
		}
		if issue.ID == int64(1) {
			assert.Equal(t, int64(400), issue.TotalTrackedTime)
			assert.NotNil(t, issue.Project)
			assert.Equal(t, int64(1), issue.Project.ID)
		} else {
			assert.Nil(t, issue.Project)
		}
	}
}

func TestIssueList_BlockingDependenciesMap(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 20}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 21}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 22}),
	}

	blockingDependenciesMap, err := issueList.BlockingDependenciesMap(db.DefaultContext)
	assert.NoError(t, err)
	if assert.Len(t, blockingDependenciesMap, 2) {
		var keys []int64
		for k := range blockingDependenciesMap {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		assert.EqualValues(t, []int64{20, 22}, keys)

		if assert.Len(t, blockingDependenciesMap[20], 1) {
			expectIssuesDependencyInfo(t,
				&issues_model.DependencyInfo{
					IssueID:      21,
					DependencyID: 20,
					Issue:        issues_model.Issue{ID: 21},
					Repository:   repo_model.Repository{ID: 60},
				},
				blockingDependenciesMap[20][0])
		}
		if assert.Len(t, blockingDependenciesMap[22], 2) {
			list := sortIssuesDependencyInfos(blockingDependenciesMap[22])
			expectIssuesDependencyInfo(t, &issues_model.DependencyInfo{
				IssueID:      20,
				DependencyID: 22,
				Issue:        issues_model.Issue{ID: 20},
				Repository:   repo_model.Repository{ID: 23},
			}, list[0])
			expectIssuesDependencyInfo(t, &issues_model.DependencyInfo{
				IssueID:      21,
				DependencyID: 22,
				Issue:        issues_model.Issue{ID: 21},
				Repository:   repo_model.Repository{ID: 60},
			}, list[1])
		}
	}

	issueList = issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 21}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 22}),
	}

	blockingDependenciesMap, err = issueList.BlockingDependenciesMap(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, blockingDependenciesMap, 1)
	assert.Len(t, blockingDependenciesMap[22], 2)
}

func TestIssueList_BlockedByDependenciesMap(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 20}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 21}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 22}),
	}

	blockedByDependenciesMap, err := issueList.BlockedByDependenciesMap(db.DefaultContext)
	assert.NoError(t, err)
	if assert.Len(t, blockedByDependenciesMap, 2) {
		var keys []int64
		for k := range blockedByDependenciesMap {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		assert.EqualValues(t, []int64{20, 21}, keys)

		if assert.Len(t, blockedByDependenciesMap[20], 1) {
			expectIssuesDependencyInfo(t,
				&issues_model.DependencyInfo{
					IssueID:      20,
					DependencyID: 22,
					Issue:        issues_model.Issue{ID: 22},
					Repository:   repo_model.Repository{ID: 61},
				},
				blockedByDependenciesMap[20][0])
		}
		if assert.Len(t, blockedByDependenciesMap[21], 2) {
			list := sortIssuesDependencyInfos(blockedByDependenciesMap[21])
			expectIssuesDependencyInfo(t, &issues_model.DependencyInfo{
				IssueID:      21,
				DependencyID: 20,
				Issue:        issues_model.Issue{ID: 20},
				Repository:   repo_model.Repository{ID: 23},
			}, list[0])
			expectIssuesDependencyInfo(t, &issues_model.DependencyInfo{
				IssueID:      21,
				DependencyID: 22,
				Issue:        issues_model.Issue{ID: 22},
				Repository:   repo_model.Repository{ID: 61},
			}, list[1])
		}
	}

	issueList = issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 21}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 22}),
	}

	blockedByDependenciesMap, err = issueList.BlockedByDependenciesMap(db.DefaultContext)
	assert.NoError(t, err)
	assert.Len(t, blockedByDependenciesMap, 1)
	assert.Len(t, blockedByDependenciesMap[21], 2)
}

func expectIssuesDependencyInfo(t *testing.T, expect, got *issues_model.DependencyInfo) {
	if expect == nil {
		assert.Nil(t, got)
		return
	}
	if !assert.NotNil(t, got) {
		return
	}
	assert.EqualValues(t, expect.DependencyID, got.DependencyID, "DependencyID")
	assert.EqualValues(t, expect.IssueID, got.IssueID, "IssueID")
	assert.EqualValues(t, expect.Issue.ID, got.Issue.ID, "RelatedIssueID")
	assert.EqualValues(t, expect.Repository.ID, got.Repository.ID, "RelatedIssueRepoID")
}

func sortIssuesDependencyInfos(in []*issues_model.DependencyInfo) []*issues_model.DependencyInfo {
	slices.SortFunc(in, func(a, b *issues_model.DependencyInfo) int {
		return cmp.Compare(a.DependencyID, b.DependencyID)
	})
	return in
}
