// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repository"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func Test_Projects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	t.Run("User projects", func(t *testing.T) {
		projects, err := db.Find[project_model.Project](db.DefaultContext, project_model.SearchOptions{
			OwnerID: user2.ID,
		})
		assert.NoError(t, err)
		assert.Len(t, projects, 3)
		assert.EqualValues(t, 4, projects[0].ID)

		t.Run("Authenticated user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				Owner:      user2,
				AccessUser: user2,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)    // 4 has 2 issues, 6 will not contains here because 0 issues
			assert.Len(t, columnIssues[4], 2) // user2 can visit both issues, one from public repository one from private repository
		})

		t.Run("Anonymous user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				AllPublic: true,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[4], 1) // anonymous user can only visit public repo issues
		})

		t.Run("Authenticated user with no permission to the private repo", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				Owner:      user2,
				AccessUser: user4,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[4], 1) // user4 can only visit public repo issues
		})
	})

	t.Run("Org projects", func(t *testing.T) {
		projects, err := db.Find[project_model.Project](db.DefaultContext, project_model.SearchOptions{
			OwnerID: org3.ID,
		})
		assert.NoError(t, err)
		assert.Len(t, projects, 1)
		assert.EqualValues(t, 7, projects[0].ID)

		t.Run("Authenticated user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				Org:        org3,
				AccessUser: user2,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)    // 4 has 2 issues, 6 will not contains here because 0 issues
			assert.Len(t, columnIssues[4], 2) // user2 can visit both issues, one from public repository one from private repository
		})

		t.Run("Anonymous user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				AllPublic: true,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[4], 1) // anonymous user can only visit public repo issues
		})

		t.Run("Authenticated user with no permission to the private repo", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				Org:        org3,
				AccessUser: user4,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[4], 1) // user4 can only visit public repo issues
		})
	})

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	t.Run("Repository projects", func(t *testing.T) {
		projects, err := db.Find[project_model.Project](db.DefaultContext, project_model.SearchOptions{
			RepoID: repo1.ID,
		})
		assert.NoError(t, err)
		assert.Len(t, projects, 1)
		assert.EqualValues(t, 1, projects[0].ID)

		t.Run("Authenticated user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				RepoIDs:    []int64{repo1.ID},
				AccessUser: user2,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)    // 4 has 2 issues, 6 will not contains here because 0 issues
			assert.Len(t, columnIssues[4], 2) // user2 can visit both issues, one from public repository one from private repository
		})

		t.Run("Anonymous user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				RepoIDs:   []int64{repo1.ID},
				AllPublic: true,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[4], 1) // anonymous user can only visit public repo issues
		})

		t.Run("Authenticated user with no permission to the private repo", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				RepoIDs:    []int64{repo1.ID},
				AccessUser: user4,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[4], 1) // user4 can only visit public repo issues
		})
	})
}
