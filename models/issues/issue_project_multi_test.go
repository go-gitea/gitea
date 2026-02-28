// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"fmt"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueLoadMultipleProjects(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Get test data
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	project1 := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})

	// Create a second project for the same repository
	project2 := &project_model.Project{
		Title:        "Test Project 2",
		RepoID:       issue1.RepoID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeBasicKanban,
	}
	require.NoError(t, project_model.NewProject(t.Context(), project2))
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project2.ID)
	}()

	// Get default columns for both projects
	columns1, err := project1.GetColumns(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, columns1)

	columns2, err := project2.GetColumns(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, columns2)

	// Assign issue to both projects
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{project1.ID, project2.ID}, columns1[0].ID)
	require.NoError(t, err)

	// Clear the projects field to force reload
	issue1.Projects = nil

	// Load projects for the issue
	err = issue1.LoadProjects(t.Context())
	require.NoError(t, err)

	// Verify issue is associated with both projects
	assert.Len(t, issue1.Projects, 2)
	projectIDs := make(map[int64]bool)
	for _, p := range issue1.Projects {
		projectIDs[p.ID] = true
	}
	assert.True(t, projectIDs[project1.ID], "Issue should be in project1")
	assert.True(t, projectIDs[project2.ID], "Issue should be in project2")

	// Clean up - remove issue from both projects
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{}, 0)
	require.NoError(t, err)
}

func TestIssueAssignMultipleProjectsSimultaneously(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Get test data
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Create three projects for the same repository
	var projects []*project_model.Project
	for i := 1; i <= 3; i++ {
		project := &project_model.Project{
			Title:        fmt.Sprintf("Test Project %d", i),
			RepoID:       issue1.RepoID,
			Type:         project_model.TypeRepository,
			TemplateType: project_model.TemplateTypeBasicKanban,
		}
		require.NoError(t, project_model.NewProject(t.Context(), project))
		projects = append(projects, project)
		defer func(id int64) {
			_ = project_model.DeleteProjectByID(t.Context(), id)
		}(project.ID)
	}

	// Get default column from first project
	columns, err := projects[0].GetColumns(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, columns)

	// Assign issue to all three projects at once
	projectIDs := make([]int64, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
	}
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, projectIDs, columns[0].ID)
	require.NoError(t, err)

	// Reload the issue and verify it's in all three projects
	issue1.Projects = nil
	err = issue1.LoadProjects(t.Context())
	require.NoError(t, err)

	assert.Len(t, issue1.Projects, 3, "Issue should be assigned to all 3 projects")

	// Verify all project IDs are present
	foundProjectIDs := make(map[int64]bool)
	for _, p := range issue1.Projects {
		foundProjectIDs[p.ID] = true
	}
	for _, p := range projects {
		assert.True(t, foundProjectIDs[p.ID], "Issue should be in project %d", p.ID)
	}

	// Clean up
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{}, 0)
	require.NoError(t, err)
}

func TestIssueRemoveFromOneProjectKeepOthers(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Get test data
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Create three projects
	var projects []*project_model.Project
	for i := 1; i <= 3; i++ {
		project := &project_model.Project{
			Title:        fmt.Sprintf("Test Project %d", i),
			RepoID:       issue1.RepoID,
			Type:         project_model.TypeRepository,
			TemplateType: project_model.TemplateTypeBasicKanban,
		}
		require.NoError(t, project_model.NewProject(t.Context(), project))
		projects = append(projects, project)
		defer func(id int64) {
			_ = project_model.DeleteProjectByID(t.Context(), id)
		}(project.ID)
	}

	// Get default column
	columns, err := projects[0].GetColumns(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, columns)

	// Assign issue to all three projects
	allProjectIDs := []int64{projects[0].ID, projects[1].ID, projects[2].ID}
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, allProjectIDs, columns[0].ID)
	require.NoError(t, err)

	// Verify issue is in all three projects
	issue1.Projects = nil
	err = issue1.LoadProjects(t.Context())
	require.NoError(t, err)
	assert.Len(t, issue1.Projects, 3, "Issue should initially be in all 3 projects")

	// Remove issue from project 2 (middle one), keep it in projects 1 and 3
	remainingProjectIDs := []int64{projects[0].ID, projects[2].ID}
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, remainingProjectIDs, columns[0].ID)
	require.NoError(t, err)

	// Reload and verify issue is only in projects 1 and 3
	issue1.Projects = nil
	issue1.ResetAttributesLoaded()
	err = issue1.LoadProjects(t.Context())
	require.NoError(t, err)
	assert.Len(t, issue1.Projects, 2, "Issue should be in only 2 projects after removal")

	// Verify the correct projects remain
	foundProjectIDs := make(map[int64]bool)
	for _, p := range issue1.Projects {
		foundProjectIDs[p.ID] = true
	}
	assert.True(t, foundProjectIDs[projects[0].ID], "Issue should still be in project 1")
	assert.False(t, foundProjectIDs[projects[1].ID], "Issue should NOT be in project 2")
	assert.True(t, foundProjectIDs[projects[2].ID], "Issue should still be in project 3")

	// Clean up
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{}, 0)
	require.NoError(t, err)
}

func TestIssueQueryByMultipleProjectIDs(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Get test data
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Create three projects
	var projects []*project_model.Project
	for i := 1; i <= 3; i++ {
		project := &project_model.Project{
			Title:        fmt.Sprintf("Query Test Project %d", i),
			RepoID:       issue1.RepoID,
			Type:         project_model.TypeRepository,
			TemplateType: project_model.TemplateTypeBasicKanban,
		}
		require.NoError(t, project_model.NewProject(t.Context(), project))
		projects = append(projects, project)
		defer func(id int64) {
			_ = project_model.DeleteProjectByID(t.Context(), id)
		}(project.ID)
	}

	// Get default column
	columns, err := projects[0].GetColumns(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, columns)

	// Assign issue1 to projects 1 and 2
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{projects[0].ID, projects[1].ID}, columns[0].ID)
	require.NoError(t, err)

	// Assign issue2 to project 3
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue2, user2, []int64{projects[2].ID}, columns[0].ID)
	require.NoError(t, err)

	// Query for issues in projects 1 and 2 (should find issue1)
	issues, err := issues_model.Issues(t.Context(), &issues_model.IssuesOptions{
		RepoIDs:    []int64{issue1.RepoID},
		ProjectIDs: []int64{projects[0].ID, projects[1].ID},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, issues, "Should find issues in projects 1 and 2")

	// Verify issue1 is in the results
	foundIssue1 := false
	for _, issue := range issues {
		if issue.ID == issue1.ID {
			foundIssue1 = true
			break
		}
	}
	assert.True(t, foundIssue1, "Issue 1 should be found when querying projects 1 and 2")

	// Query for issues in project 3 only (should find issue2)
	issues, err = issues_model.Issues(t.Context(), &issues_model.IssuesOptions{
		RepoIDs:    []int64{issue1.RepoID},
		ProjectIDs: []int64{projects[2].ID},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, issues, "Should find issues in project 3")

	// Verify issue2 is in the results
	foundIssue2 := false
	for _, issue := range issues {
		if issue.ID == issue2.ID {
			foundIssue2 = true
			break
		}
	}
	assert.True(t, foundIssue2, "Issue 2 should be found when querying project 3")

	// Query for all three projects (should find both issues)
	issues, err = issues_model.Issues(t.Context(), &issues_model.IssuesOptions{
		RepoIDs:    []int64{issue1.RepoID},
		ProjectIDs: []int64{projects[0].ID, projects[1].ID, projects[2].ID},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, issues, "Should find issues in all three projects")

	foundIssue1 = false
	foundIssue2 = false
	for _, issue := range issues {
		if issue.ID == issue1.ID {
			foundIssue1 = true
		}
		if issue.ID == issue2.ID {
			foundIssue2 = true
		}
	}
	assert.True(t, foundIssue1, "Issue 1 should be found when querying all projects")
	assert.True(t, foundIssue2, "Issue 2 should be found when querying all projects")

	// Clean up
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{}, 0)
	require.NoError(t, err)
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue2, user2, []int64{}, 0)
	require.NoError(t, err)
}

func TestIssueBackwardCompatibilitySingleProject(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// Get test data
	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Create a single project
	project := &project_model.Project{
		Title:        "Backward Compat Test Project",
		RepoID:       issue1.RepoID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeBasicKanban,
	}
	require.NoError(t, project_model.NewProject(t.Context(), project))
	defer func() {
		_ = project_model.DeleteProjectByID(t.Context(), project.ID)
	}()

	// Get default column
	columns, err := project.GetColumns(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, columns)

	// Test assigning to a single project (old style behavior with single ID in array)
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{project.ID}, columns[0].ID)
	require.NoError(t, err)

	// Load projects and verify issue is in exactly one project
	issue1.Projects = nil
	err = issue1.LoadProjects(t.Context())
	require.NoError(t, err)
	assert.Len(t, issue1.Projects, 1, "Issue should be in exactly 1 project")
	assert.Equal(t, project.ID, issue1.Projects[0].ID, "Issue should be in the correct project")

	// Test removing from a single project
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{}, 0)
	require.NoError(t, err)

	// Verify issue is no longer in any project
	issue1.Projects = nil
	err = issue1.LoadProjects(t.Context())
	require.NoError(t, err)
	assert.Empty(t, issue1.Projects, "Issue should not be in any project after removal")

	// Test querying with a single project ID
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{project.ID}, columns[0].ID)
	require.NoError(t, err)

	issues, err := issues_model.Issues(t.Context(), &issues_model.IssuesOptions{
		RepoIDs:    []int64{issue1.RepoID},
		ProjectIDs: []int64{project.ID},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, issues, "Should find issues when querying a single project")

	foundIssue1 := false
	for _, issue := range issues {
		if issue.ID == issue1.ID {
			foundIssue1 = true
			break
		}
	}
	assert.True(t, foundIssue1, "Issue should be found when querying single project")

	// Clean up
	err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{}, 0)
	require.NoError(t, err)
}
