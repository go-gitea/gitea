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

func TestIssueMultipleProjects(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("GeneralTest", func(t *testing.T) {
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

		err := issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{})
		require.NoError(t, err)
		err = issue1.LoadProjects(t.Context())
		require.NoError(t, err)
		require.Empty(t, issue1.Projects)

		// assign issue to both projects (each project uses its own default column)
		err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{project1.ID})
		require.NoError(t, err)
		assert.Nilf(t, issue1.Projects, "Issue's Projects should be nil after IssueAssignOrRemoveProject to ensure it reloads fresh data")
		err = issue1.LoadProjects(t.Context())
		require.NoError(t, err)
		require.Len(t, issue1.Projects, 1)

		err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{project1.ID, project2.ID})
		require.NoError(t, err)
		assert.Nilf(t, issue1.Projects, "Issue's Projects should be nil after IssueAssignOrRemoveProject to ensure it reloads fresh data")
		err = issue1.LoadProjects(t.Context())
		require.NoError(t, err)
		require.Len(t, issue1.Projects, 2)
		assert.ElementsMatch(t, []int64{project1.ID, project2.ID}, []int64{issue1.Projects[0].ID, issue1.Projects[1].ID}, "Issue should be in both projects")

		// test issue's project column map
		projectColumnMap, err := issue1.ProjectColumnMap(t.Context())
		p1Col, _ := project1.MustDefaultColumn(t.Context())
		p2Col, _ := project2.MustDefaultColumn(t.Context())
		require.NoError(t, err)
		assert.Equal(t, p1Col.ID, projectColumnMap[project1.ID])
		assert.Equal(t, p2Col.ID, projectColumnMap[project2.ID])

		// only keep project2
		err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{project2.ID})
		require.NoError(t, err)
		err = issue1.LoadProjects(t.Context())
		require.NoError(t, err)
		require.Len(t, issue1.Projects, 1)
		assert.Equal(t, project2.ID, issue1.Projects[0].ID)

		// also test ResetAttributesLoaded
		issue1.Projects = nil
		issue1.ResetAttributesLoaded()
		err = issue1.LoadProjects(t.Context())
		require.NoError(t, err)
		require.Len(t, issue1.Projects, 1)
		assert.Equal(t, project2.ID, issue1.Projects[0].ID)

		// remove issue's projects
		err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{})
		require.NoError(t, err)
		err = issue1.LoadProjects(t.Context())
		require.NoError(t, err)
		require.Empty(t, issue1.Projects)
	})

	t.Run("QueryByMultipleProjectIDs", func(t *testing.T) {
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

		// Assign issue1 to projects 1 and 2
		err := issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{projects[0].ID, projects[1].ID})
		require.NoError(t, err)

		// Assign issue2 to project 3
		err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue2, user2, []int64{projects[2].ID})
		require.NoError(t, err)

		// Query for issues in project 3 only (should find issue2)
		issues, err := issues_model.Issues(t.Context(), &issues_model.IssuesOptions{
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

		// FIXME: ISSUE-MULTIPLE-PROJECTS-FILTER: no multiple project filter support yet. Search logic is wrong. It should use "AND" but not "OR".
		// Clean up
		err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue1, user2, []int64{})
		require.NoError(t, err)
		err = issues_model.IssueAssignOrRemoveProject(t.Context(), issue2, user2, []int64{})
		require.NoError(t, err)
	})
}
