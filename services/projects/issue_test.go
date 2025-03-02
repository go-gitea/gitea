// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	org_model "code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func Test_Projects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	userAdmin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &org_model.Organization{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	t.Run("User projects", func(t *testing.T) {
		pi1 := project_model.ProjectIssue{
			ProjectID:       4,
			IssueID:         1,
			ProjectColumnID: 4,
		}
		err := db.Insert(db.DefaultContext, &pi1)
		assert.NoError(t, err)
		defer func() {
			_, err = db.DeleteByID[project_model.ProjectIssue](db.DefaultContext, pi1.ID)
			assert.NoError(t, err)
		}()

		pi2 := project_model.ProjectIssue{
			ProjectID:       4,
			IssueID:         4,
			ProjectColumnID: 4,
		}
		err = db.Insert(db.DefaultContext, &pi2)
		assert.NoError(t, err)
		defer func() {
			_, err = db.DeleteByID[project_model.ProjectIssue](db.DefaultContext, pi2.ID)
			assert.NoError(t, err)
		}()

		projects, err := db.Find[project_model.Project](db.DefaultContext, project_model.SearchOptions{
			OwnerID: user2.ID,
		})
		assert.NoError(t, err)
		assert.Len(t, projects, 3)
		assert.EqualValues(t, 4, projects[0].ID)

		t.Run("Authenticated user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				Owner: user2,
				Doer:  user2,
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
				Owner: user2,
				Doer:  user4,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[4], 1) // user4 can only visit public repo issues
		})
	})

	t.Run("Org projects", func(t *testing.T) {
		project1 := project_model.Project{
			Title:        "project in an org",
			OwnerID:      org3.ID,
			Type:         project_model.TypeOrganization,
			TemplateType: project_model.TemplateTypeBasicKanban,
		}
		err := project_model.NewProject(db.DefaultContext, &project1)
		assert.NoError(t, err)
		defer func() {
			err := project_model.DeleteProjectByID(db.DefaultContext, project1.ID)
			assert.NoError(t, err)
		}()

		column1 := project_model.Column{
			Title:     "column 1",
			ProjectID: project1.ID,
		}
		err = project_model.NewColumn(db.DefaultContext, &column1)
		assert.NoError(t, err)

		column2 := project_model.Column{
			Title:     "column 2",
			ProjectID: project1.ID,
		}
		err = project_model.NewColumn(db.DefaultContext, &column2)
		assert.NoError(t, err)

		// issue 6 belongs to private repo 3 under org 3
		issue6 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 6})
		err = issues_model.IssueAssignOrRemoveProject(db.DefaultContext, issue6, user2, project1.ID, column1.ID)
		assert.NoError(t, err)

		// issue 16 belongs to public repo 16 under org 3
		issue16 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 16})
		err = issues_model.IssueAssignOrRemoveProject(db.DefaultContext, issue16, user2, project1.ID, column1.ID)
		assert.NoError(t, err)

		projects, err := db.Find[project_model.Project](db.DefaultContext, project_model.SearchOptions{
			OwnerID: org3.ID,
		})
		assert.NoError(t, err)
		assert.Len(t, projects, 1)
		assert.EqualValues(t, project1.ID, projects[0].ID)

		t.Run("Authenticated user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				Owner: org3.AsUser(),
				Doer:  userAdmin,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)             // column1 has 2 issues, 6 will not contains here because 0 issues
			assert.Len(t, columnIssues[column1.ID], 2) // user2 can visit both issues, one from public repository one from private repository
		})

		t.Run("Anonymous user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				AllPublic: true,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[column1.ID], 1) // anonymous user can only visit public repo issues
		})

		t.Run("Authenticated user with no permission to the private repo", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				Owner: org3.AsUser(),
				Doer:  user2,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 1)
			assert.Len(t, columnIssues[column1.ID], 1) // user4 can only visit public repo issues
		})
	})

	t.Run("Repository projects", func(t *testing.T) {
		repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

		projects, err := db.Find[project_model.Project](db.DefaultContext, project_model.SearchOptions{
			RepoID: repo1.ID,
		})
		assert.NoError(t, err)
		assert.Len(t, projects, 1)
		assert.EqualValues(t, 1, projects[0].ID)

		t.Run("Authenticated user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				RepoIDs: []int64{repo1.ID},
				Doer:    userAdmin,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 3)
			assert.Len(t, columnIssues[1], 2)
			assert.Len(t, columnIssues[2], 1)
			assert.Len(t, columnIssues[3], 1)
		})

		t.Run("Anonymous user", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				AllPublic: true,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 3)
			assert.Len(t, columnIssues[1], 2)
			assert.Len(t, columnIssues[2], 1)
			assert.Len(t, columnIssues[3], 1)
		})

		t.Run("Authenticated user with no permission to the private repo", func(t *testing.T) {
			columnIssues, err := LoadIssuesFromProject(db.DefaultContext, projects[0], &issues_model.IssuesOptions{
				RepoIDs: []int64{repo1.ID},
				Doer:    user2,
			})
			assert.NoError(t, err)
			assert.Len(t, columnIssues, 3)
			assert.Len(t, columnIssues[1], 2)
			assert.Len(t, columnIssues[2], 1)
			assert.Len(t, columnIssues[3], 1)
		})
	})
}
