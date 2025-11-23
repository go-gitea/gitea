// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIListProjects(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeReadRepository)

	// Test listing all projects
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects", owner.Name, repo.Name).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var projects []*api.Project
	DecodeJSON(t, resp, &projects)
	assert.GreaterOrEqual(t, len(projects), 0)

	// Test state filter - open
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects?state=open", owner.Name, repo.Name).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &projects)
	for _, project := range projects {
		assert.False(t, project.IsClosed, "Project should be open")
	}

	// Test state filter - all
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects?state=all", owner.Name, repo.Name).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &projects)
	assert.GreaterOrEqual(t, len(projects), 0)

	// Test pagination
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects?page=1&limit=5", owner.Name, repo.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
}

func TestAPIGetProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// Create a test project
	project := &project_model.Project{
		Title:        "Test Project for API",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeReadRepository)

	// Test getting the project
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiProject api.Project
	DecodeJSON(t, resp, &apiProject)
	assert.Equal(t, project.Title, apiProject.Title)
	assert.Equal(t, project.ID, apiProject.ID)
	assert.Equal(t, repo.ID, int64(apiProject.RepoID))
	assert.NotEmpty(t, apiProject.URL)

	// Test getting non-existent project
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects/99999", owner.Name, repo.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPICreateProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test creating a project
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects", owner.Name, repo.Name), &api.CreateProjectOption{
		Title:        "API Created Project",
		Description:  "This is a test project created via API",
		TemplateType: 1, // basic_kanban
		CardType:     1, // images_and_text
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	var project api.Project
	DecodeJSON(t, resp, &project)
	assert.Equal(t, "API Created Project", project.Title)
	assert.Equal(t, "This is a test project created via API", project.Description)
	assert.Equal(t, 1, project.TemplateType)
	assert.Equal(t, 1, project.CardType)
	assert.False(t, project.IsClosed)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	// Test creating with minimal data
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects", owner.Name, repo.Name), &api.CreateProjectOption{
		Title: "Minimal Project",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)

	var minimalProject api.Project
	DecodeJSON(t, resp, &minimalProject)
	assert.Equal(t, "Minimal Project", minimalProject.Title)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), minimalProject.ID)
	}()

	// Test creating without authentication
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects", owner.Name, repo.Name), &api.CreateProjectOption{
		Title: "Unauthorized Project",
	})
	MakeRequest(t, req, http.StatusUnauthorized)

	// Test creating with invalid data (empty title)
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects", owner.Name, repo.Name), &api.CreateProjectOption{
		Title: "",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPIUpdateProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// Create a test project
	project := &project_model.Project{
		Title:        "Project to Update",
		Description:  "Original description",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test updating project title and description
	newTitle := "Updated Project Title"
	newDesc := "Updated description"
	req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID), &api.EditProjectOption{
		Title:       &newTitle,
		Description: &newDesc,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var updatedProject api.Project
	DecodeJSON(t, resp, &updatedProject)
	assert.Equal(t, newTitle, updatedProject.Title)
	assert.Equal(t, newDesc, updatedProject.Description)

	// Test closing project
	isClosed := true
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID), &api.EditProjectOption{
		IsClosed: &isClosed,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &updatedProject)
	assert.True(t, updatedProject.IsClosed)
	assert.NotNil(t, updatedProject.ClosedDate)

	// Test reopening project
	isClosed = false
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID), &api.EditProjectOption{
		IsClosed: &isClosed,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &updatedProject)
	assert.False(t, updatedProject.IsClosed)

	// Test updating non-existent project
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/99999", owner.Name, repo.Name), &api.EditProjectOption{
		Title: &newTitle,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIDeleteProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// Create a test project
	project := &project_model.Project{
		Title:        "Project to Delete",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test deleting the project
	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Verify project is deleted
	exists, err := project_model.ExistsProjectByID(t.Context(), project.ID)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Test deleting non-existent project
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/projects/99999", owner.Name, repo.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIListProjectColumns(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// Create a test project
	project := &project_model.Project{
		Title:        "Project for Columns Test",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	// Create test columns
	for i := 1; i <= 3; i++ {
		column := &project_model.Column{
			Title:     fmt.Sprintf("Column %d", i),
			ProjectID: project.ID,
			CreatorID: owner.ID,
		}
		err = project_model.NewColumn(t.Context(), column)
		assert.NoError(t, err)
	}

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeReadRepository)

	// Test listing columns
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects/%d/columns", owner.Name, repo.Name, project.ID).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var columns []*api.ProjectColumn
	DecodeJSON(t, resp, &columns)
	assert.Len(t, columns, 3)
	assert.Equal(t, "Column 1", columns[0].Title)
	assert.Equal(t, "Column 2", columns[1].Title)
	assert.Equal(t, "Column 3", columns[2].Title)

	// Test pagination
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects/%d/columns?page=1&limit=2", owner.Name, repo.Name, project.ID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &columns)
	assert.Len(t, columns, 2)

	// Test listing columns for non-existent project
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects/99999/columns", owner.Name, repo.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPICreateProjectColumn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// Create a test project
	project := &project_model.Project{
		Title:        "Project for Column Creation",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test creating a column with color
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d/columns", owner.Name, repo.Name, project.ID), &api.CreateProjectColumnOption{
		Title: "New Column",
		Color: "#FF5733",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	var column api.ProjectColumn
	DecodeJSON(t, resp, &column)
	assert.Equal(t, "New Column", column.Title)
	assert.Equal(t, "#FF5733", column.Color)
	assert.Equal(t, project.ID, column.ProjectID)

	// Test creating a column without color
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d/columns", owner.Name, repo.Name, project.ID), &api.CreateProjectColumnOption{
		Title: "Simple Column",
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)

	DecodeJSON(t, resp, &column)
	assert.Equal(t, "Simple Column", column.Title)

	// Test creating with empty title
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d/columns", owner.Name, repo.Name, project.ID), &api.CreateProjectColumnOption{
		Title: "",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test creating for non-existent project
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/99999/columns", owner.Name, repo.Name), &api.CreateProjectColumnOption{
		Title: "Orphan Column",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIUpdateProjectColumn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// Create a test project and column
	project := &project_model.Project{
		Title:        "Project for Column Update",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	column := &project_model.Column{
		Title:     "Original Column",
		ProjectID: project.ID,
		CreatorID: owner.ID,
		Color:     "#000000",
	}
	err = project_model.NewColumn(t.Context(), column)
	assert.NoError(t, err)

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test updating column title
	newTitle := "Updated Column"
	req := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/%d", owner.Name, repo.Name, column.ID), &api.EditProjectColumnOption{
		Title: &newTitle,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var updatedColumn api.ProjectColumn
	DecodeJSON(t, resp, &updatedColumn)
	assert.Equal(t, newTitle, updatedColumn.Title)

	// Test updating column color
	newColor := "#FF0000"
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/%d", owner.Name, repo.Name, column.ID), &api.EditProjectColumnOption{
		Color: &newColor,
	}).AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &updatedColumn)
	assert.Equal(t, newColor, updatedColumn.Color)

	// Test updating non-existent column
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/99999", owner.Name, repo.Name), &api.EditProjectColumnOption{
		Title: &newTitle,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIDeleteProjectColumn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// Create a test project and column
	project := &project_model.Project{
		Title:        "Project for Column Deletion",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	column := &project_model.Column{
		Title:     "Column to Delete",
		ProjectID: project.ID,
		CreatorID: owner.ID,
	}
	err = project_model.NewColumn(t.Context(), column)
	assert.NoError(t, err)

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test deleting the column
	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/projects/columns/%d", owner.Name, repo.Name, column.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Verify column is deleted
	exists, err := project_model.ExistsColumnByID(t.Context(), column.ID)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Test deleting non-existent column
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/projects/columns/99999", owner.Name, repo.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIAddIssueToProjectColumn(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})

	// Create a test project and column
	project := &project_model.Project{
		Title:        "Project for Issue Assignment",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	column1 := &project_model.Column{
		Title:     "Column 1",
		ProjectID: project.ID,
		CreatorID: owner.ID,
	}
	err = project_model.NewColumn(t.Context(), column1)
	assert.NoError(t, err)

	column2 := &project_model.Column{
		Title:     "Column 2",
		ProjectID: project.ID,
		CreatorID: owner.ID,
	}
	err = project_model.NewColumn(t.Context(), column2)
	assert.NoError(t, err)

	token := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)

	// Test adding issue to column
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/%d/issues", owner.Name, repo.Name, column1.ID), &api.AddIssueToProjectColumnOption{
		IssueID: issue.ID,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// Verify issue is in the column
	projectIssue, err := project_model.GetProjectIssue(t.Context(), issue.ID)
	assert.NoError(t, err)
	assert.NotNil(t, projectIssue)
	assert.Equal(t, column1.ID, projectIssue.ProjectColumnID)

	// Test moving issue to another column
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/%d/issues", owner.Name, repo.Name, column2.ID), &api.AddIssueToProjectColumnOption{
		IssueID: issue.ID,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// Verify issue moved to new column
	projectIssue, err = project_model.GetProjectIssue(t.Context(), issue.ID)
	assert.NoError(t, err)
	assert.Equal(t, column2.ID, projectIssue.ProjectColumnID)

	// Test adding same issue to same column (should be idempotent)
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/%d/issues", owner.Name, repo.Name, column2.ID), &api.AddIssueToProjectColumnOption{
		IssueID: issue.ID,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	// Test adding non-existent issue
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/%d/issues", owner.Name, repo.Name, column1.ID), &api.AddIssueToProjectColumnOption{
		IssueID: 99999,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test adding to non-existent column
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/projects/columns/99999/issues", owner.Name, repo.Name), &api.AddIssueToProjectColumnOption{
		IssueID: issue.ID,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIProjectPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"})

	// Create a test project
	project := &project_model.Project{
		Title:        "Permission Test Project",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		CreatorID:    owner.ID,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	ownerToken := getUserToken(t, owner.Name, auth_model.AccessTokenScopeWriteRepository)
	user2Token := getUserToken(t, user2.Name, auth_model.AccessTokenScopeWriteRepository)

	// Owner should be able to read
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID).
		AddTokenAuth(ownerToken)
	MakeRequest(t, req, http.StatusOK)

	// Owner should be able to update
	newTitle := "Updated by Owner"
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID), &api.EditProjectOption{
		Title: &newTitle,
	}).AddTokenAuth(ownerToken)
	MakeRequest(t, req, http.StatusOK)

	// User2 (non-collaborator) should not be able to update
	anotherTitle := "Updated by User2"
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID), &api.EditProjectOption{
		Title: &anotherTitle,
	}).AddTokenAuth(user2Token)
	MakeRequest(t, req, http.StatusForbidden)

	// User2 should not be able to delete
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/projects/%d", owner.Name, repo.Name, project.ID).
		AddTokenAuth(user2Token)
	MakeRequest(t, req, http.StatusForbidden)
}
