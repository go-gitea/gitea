// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivateRepoProject(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// not logged in user
	req := NewRequest(t, "GET", "/user31/-/projects")
	MakeRequest(t, req, http.StatusNotFound)

	sess := loginUser(t, "user1")
	req = NewRequest(t, "GET", "/user31/-/projects")
	sess.MakeRequest(t, req, http.StatusOK)
}

func TestMoveRepoProjectColumns(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	projectsUnit := repo2.MustGetUnit(t.Context(), unit.TypeProjects)
	assert.True(t, projectsUnit.ProjectsConfig().IsProjectsAllowed(repo_model.ProjectsModeRepo))

	project1 := project_model.Project{
		Title:        "new created project",
		RepoID:       repo2.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), &project1)
	assert.NoError(t, err)

	for i := range 3 {
		err = project_model.NewColumn(t.Context(), &project_model.Column{
			Title:     fmt.Sprintf("column %d", i+1),
			ProjectID: project1.ID,
		})
		assert.NoError(t, err)
	}

	columns, err := project1.GetColumns(t.Context())
	assert.NoError(t, err)
	assert.Len(t, columns, 3)
	assert.EqualValues(t, 0, columns[0].Sorting)
	assert.EqualValues(t, 1, columns[1].Sorting)
	assert.EqualValues(t, 2, columns[2].Sorting)

	sess := loginUser(t, "user1")
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/projects/%d", repo2.FullName(), project1.ID))
	sess.MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s/projects/%d/move", repo2.FullName(), project1.ID), map[string]any{
		"columns": []map[string]any{
			{"columnID": columns[1].ID, "sorting": 0},
			{"columnID": columns[2].ID, "sorting": 1},
			{"columnID": columns[0].ID, "sorting": 2},
		},
	})
	sess.MakeRequest(t, req, http.StatusOK)

	columnsAfter, err := project1.GetColumns(t.Context())
	assert.NoError(t, err)
	assert.Len(t, columnsAfter, 3)
	assert.Equal(t, columns[1].ID, columnsAfter[0].ID)
	assert.Equal(t, columns[2].ID, columnsAfter[1].ID)
	assert.Equal(t, columns[0].ID, columnsAfter[2].ID)

	assert.NoError(t, project_model.DeleteProjectByID(t.Context(), project1.ID))
}

// getProjectIssueIDs returns the set of issue IDs rendered as cards on the project board page.
func getProjectIssueIDs(t *testing.T, htmlDoc *HTMLDoc) map[int64]struct{} {
	t.Helper()
	ids := make(map[int64]struct{})
	htmlDoc.Find(".issue-card[data-issue]").Each(func(_ int, s *goquery.Selection) {
		idStr, exists := s.Attr("data-issue")
		require.True(t, exists)
		id, err := strconv.ParseInt(idStr, 10, 64)
		require.NoError(t, err)
		ids[id] = struct{}{}
	})
	return ids
}

func TestRepoProjectFilterByMilestone(t *testing.T) {
	// Project 1 is on repo 1 (user2/repo1) and has issues:
	//   issue 1 (milestone_id=0), issue 2 (milestone_id=1), issue 3 (milestone_id=3), issue 5 (milestone_id=0)
	defer tests.PrepareTestEnv(t)()

	sess := loginUser(t, "user2")

	t.Run("NoFilter", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/projects/1")
		resp := sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		// All issues should be visible
		assert.Contains(t, issueIDs, int64(1))
		assert.Contains(t, issueIDs, int64(2))
		assert.Contains(t, issueIDs, int64(3))
		assert.Contains(t, issueIDs, int64(5))
	})

	t.Run("FilterByMilestone", func(t *testing.T) {
		// milestone_id=1 is "milestone1" (open), only issue 2 has it
		req := NewRequest(t, "GET", "/user2/repo1/projects/1?milestone=1")
		resp := sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		assert.Contains(t, issueIDs, int64(2))
		assert.NotContains(t, issueIDs, int64(1))
		assert.NotContains(t, issueIDs, int64(3))
		assert.NotContains(t, issueIDs, int64(5))
	})

	t.Run("FilterByNoMilestone", func(t *testing.T) {
		// milestone=-1 means "no milestone", issues 1 and 5 have no milestone
		req := NewRequest(t, "GET", "/user2/repo1/projects/1?milestone=-1")
		resp := sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		assert.Contains(t, issueIDs, int64(1))
		assert.Contains(t, issueIDs, int64(5))
		assert.NotContains(t, issueIDs, int64(2))
		assert.NotContains(t, issueIDs, int64(3))
	})

	t.Run("FilterByClosedMilestone", func(t *testing.T) {
		// milestone_id=3 is "milestone3" (closed), only issue 3 has it
		req := NewRequest(t, "GET", "/user2/repo1/projects/1?milestone=3")
		resp := sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		assert.Contains(t, issueIDs, int64(3))
		assert.NotContains(t, issueIDs, int64(1))
		assert.NotContains(t, issueIDs, int64(2))
		assert.NotContains(t, issueIDs, int64(5))
	})
}

func TestOrgProjectFilterByMilestone(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// org3 owns repo32 (public) which has issues 16 and 17
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32})
	issue16 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 16})
	issue17 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 17})
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Create a milestone on repo32 and assign it to issue16
	milestone := &issues_model.Milestone{
		RepoID: repo.ID,
		Name:   "org-test-milestone",
	}
	require.NoError(t, issues_model.NewMilestone(t.Context(), milestone))

	issue16.MilestoneID = milestone.ID
	require.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue16, "milestone_id"))

	// Create an org-level project
	project := project_model.Project{
		Title:        "org milestone filter test",
		OwnerID:      org.ID,
		Type:         project_model.TypeOrganization,
		TemplateType: project_model.TemplateTypeBasicKanban,
	}
	require.NoError(t, project_model.NewProject(t.Context(), &project))

	// Get the default column
	columns, err := project.GetColumns(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, columns)
	defaultColumnID := columns[0].ID

	// Add issues to the project
	require.NoError(t, issues_model.IssueAssignOrRemoveProject(t.Context(), issue16, user1, project.ID, defaultColumnID))
	require.NoError(t, issues_model.IssueAssignOrRemoveProject(t.Context(), issue17, user1, project.ID, defaultColumnID))

	sess := loginUser(t, "user1")
	projectURL := fmt.Sprintf("/org3/-/projects/%d", project.ID)

	t.Run("NoFilter", func(t *testing.T) {
		req := NewRequest(t, "GET", projectURL)
		resp := sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		assert.Contains(t, issueIDs, issue16.ID)
		assert.Contains(t, issueIDs, issue17.ID)
	})

	t.Run("FilterByMilestone", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("%s?milestone=%d", projectURL, milestone.ID))
		resp := sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		assert.Contains(t, issueIDs, issue16.ID)
		assert.NotContains(t, issueIDs, issue17.ID)
	})

	t.Run("FilterByNoMilestone", func(t *testing.T) {
		req := NewRequest(t, "GET", projectURL+"?milestone=-1")
		resp := sess.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		assert.Contains(t, issueIDs, issue17.ID)
		assert.NotContains(t, issueIDs, issue16.ID)
	})

	t.Run("AnonymousAccess", func(t *testing.T) {
		// Anonymous users should be able to view org project boards for public orgs
		// and the milestone filter should work without exposing private repo data
		req := NewRequest(t, "GET", projectURL)
		resp := MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		issueIDs := getProjectIssueIDs(t, htmlDoc)
		// repo32 is public, so anonymous users should see its issues
		assert.Contains(t, issueIDs, issue16.ID)
		assert.Contains(t, issueIDs, issue17.ID)

		// Milestone filtering should also work for anonymous users
		req = NewRequest(t, "GET", fmt.Sprintf("%s?milestone=%d", projectURL, milestone.ID))
		resp = MakeRequest(t, req, http.StatusOK)
		htmlDoc = NewHTMLParser(t, resp.Body)
		issueIDs = getProjectIssueIDs(t, htmlDoc)
		assert.Contains(t, issueIDs, issue16.ID)
		assert.NotContains(t, issueIDs, issue17.ID)
	})
}
