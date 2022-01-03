// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestMigrate_InsertMilestones(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame}).(*repo_model.Repository)
	name := "milestonetest1"
	ms := &Milestone{
		RepoID: repo.ID,
		Name:   name,
	}
	err := InsertMilestones(ms)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, ms)
	repoModified := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID}).(*repo_model.Repository)
	assert.EqualValues(t, repo.NumMilestones+1, repoModified.NumMilestones)

	unittest.CheckConsistencyFor(t, &Milestone{})
}

func assertCreateIssues(t *testing.T, reponame string, isPull bool) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame}).(*repo_model.Repository)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)
	label := unittest.AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	milestone := unittest.AssertExistsAndLoadBean(t, &Milestone{ID: 1}).(*Milestone)
	assert.EqualValues(t, milestone.ID, 1)
	reaction := &Reaction{
		Type:   "heart",
		UserID: owner.ID,
	}

	title := "issuetitle1"
	var is = &Issue{
		RepoID:      repo.ID,
		MilestoneID: milestone.ID,
		Repo:        repo,
		Title:       title,
		Content:     "issuecontent1",
		IsPull:      isPull,
		PosterID:    owner.ID,
		Poster:      owner,
		IsClosed:    true,
		Labels:      []*Label{label},
		Reactions:   []*Reaction{reaction},
	}
	err := InsertIssues(is)
	assert.NoError(t, err)

	i := unittest.AssertExistsAndLoadBean(t, &Issue{Title: title}).(*Issue)
	unittest.AssertExistsAndLoadBean(t, &Reaction{Type: "heart", UserID: owner.ID, IssueID: i.ID})

	labelModified := unittest.AssertExistsAndLoadBean(t, &Label{ID: 1}).(*Label)
	assert.EqualValues(t, label.NumIssues+1, labelModified.NumIssues)
	assert.EqualValues(t, label.NumClosedIssues+1, labelModified.NumClosedIssues)

	milestoneModified := unittest.AssertExistsAndLoadBean(t, &Milestone{ID: milestone.ID}).(*Milestone)
	assert.EqualValues(t, milestone.NumIssues+1, milestoneModified.NumIssues)
	assert.EqualValues(t, milestone.NumClosedIssues+1, milestoneModified.NumClosedIssues)
}

func TestMigrate_CreateIssuesIsPullFalse(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame}).(*repo_model.Repository)

	assertCreateIssues(t, reponame, false)

	repoModified := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID}).(*repo_model.Repository)
	assert.EqualValues(t, repo.NumIssues+1, repoModified.NumIssues)
	assert.EqualValues(t, repo.NumClosedIssues+1, repoModified.NumClosedIssues)
}

func TestMigrate_CreateIssuesIsPullTrue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame}).(*repo_model.Repository)

	assertCreateIssues(t, reponame, true)

	repoModified := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID}).(*repo_model.Repository)
	assert.EqualValues(t, repo.NumPulls+1, repoModified.NumPulls)
	assert.EqualValues(t, repo.NumClosedPulls+1, repoModified.NumClosedPulls)
}

func TestMigrate_InsertIssueComments(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	_ = issue.LoadRepo()
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID}).(*user_model.User)
	reaction := &Reaction{
		Type:   "heart",
		UserID: owner.ID,
	}

	comment := &Comment{
		PosterID:  owner.ID,
		Poster:    owner,
		IssueID:   issue.ID,
		Issue:     issue,
		Reactions: []*Reaction{reaction},
	}

	err := InsertIssueComments([]*Comment{comment})
	assert.NoError(t, err)

	issueModified := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	assert.EqualValues(t, issue.NumComments+1, issueModified.NumComments)

	unittest.CheckConsistencyFor(t, &Issue{})
}

func TestMigrate_InsertPullRequests(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame}).(*repo_model.Repository)
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)

	var i = &Issue{
		RepoID:   repo.ID,
		Repo:     repo,
		Title:    "title1",
		Content:  "issuecontent1",
		IsPull:   true,
		PosterID: owner.ID,
		Poster:   owner,
	}

	var p = &PullRequest{
		Issue: i,
	}

	err := InsertPullRequests(p)
	assert.NoError(t, err)

	_ = unittest.AssertExistsAndLoadBean(t, &PullRequest{IssueID: i.ID}).(*PullRequest)

	unittest.CheckConsistencyFor(t, &Issue{}, &PullRequest{})
}

func TestMigrate_InsertReleases(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	a := &repo_model.Attachment{
		UUID: "a0eebc91-9c0c-4ef7-bb6e-6bb9bd380a12",
	}
	r := &Release{
		Attachments: []*repo_model.Attachment{a},
	}

	err := InsertReleases(r)
	assert.NoError(t, err)
}
