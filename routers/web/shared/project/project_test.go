// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

// TestFindColumn covers the scoping rule every board handler depends on: a request only
// resolves projects owned by the scope its route assigned, so an ID belonging to anyone
// else reads as not found rather than leaking across owners.
func TestFindColumn(t *testing.T) {
	for _, tc := range []struct {
		name       string
		projectID  string
		columnID   string
		doerID     int64
		repoScoped bool
		resolves   bool
	}{
		{"repository project", "1", "2", 2, true, true},
		{"owner project", "4", "4", 2, false, true},
		{"repository board cannot reach an owner project", "4", "1", 2, true, false},
		{"owner board cannot reach another owner's project", "4", "4", 1, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unittest.PrepareTestEnv(t)
			ctx, _ := contexttest.MockContext(t, "user2/-/projects")
			contexttest.LoadUser(t, ctx, tc.doerID)
			if tc.repoScoped {
				contexttest.LoadRepo(t, ctx, 1)
			} else {
				ctx.ContextUser = ctx.Doer
			}
			ctx.SetPathParam("id", tc.projectID)
			ctx.SetPathParam("columnID", tc.columnID)

			project, column := findColumn(ctx)
			assert.Equal(t, tc.resolves, project != nil)
			assert.Equal(t, tc.resolves, column != nil)
			if tc.resolves {
				assert.False(t, ctx.Written())
			} else {
				// a foreign ID must read as not found, never as a 500
				assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
			}
		})
	}
}

func TestMoveIssuesNeedsIssueRead(t *testing.T) {
	stripTeamIssues := func(t *testing.T) {
		_, err := db.Exec(t.Context(), "UPDATE team SET authorize = ? WHERE id = ?", perm.AccessModeNone, 2)
		require.NoError(t, err)
		_, err = db.DeleteByBean(t.Context(), &organization.TeamUnit{TeamID: 2, Type: unit.TypeIssues})
		require.NoError(t, err)
	}
	archiveRepo3 := func(t *testing.T) {
		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), &repo_model.Repository{ID: 3, IsArchived: true}, "is_archived"))
	}
	for _, tc := range []struct {
		name      string
		doerID    int64
		repoBoard bool
		prepare   func(*testing.T)
		status    int
	}{
		{"OwnerBoardReader", 2, false, nil, http.StatusOK},
		{"OwnerBoardWithoutIssueAccess", 15, false, nil, http.StatusNotFound},
		{"OwnerBoardArchivedRepo", 2, false, archiveRepo3, http.StatusLocked},
		{"RepoBoardProjectsWriterWithoutIssueRead", 4, true, stripTeamIssues, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unittest.PrepareTestEnv(t)
			if tc.prepare != nil {
				tc.prepare(t)
			}
			ctx, _ := contexttest.MockContext(t, "POST /org3/-/projects")
			contexttest.LoadUser(t, ctx, tc.doerID)
			project := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 2})
			if tc.repoBoard {
				contexttest.LoadRepo(t, ctx, 3)
			} else {
				ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
				project = &project_model.Project{Title: "board", OwnerID: 3, CreatorID: 2, Type: project_model.TypeOrganization}
				require.NoError(t, project_model.NewProject(ctx, project))
				require.NoError(t, project_model.NewColumn(ctx, &project_model.Column{Title: "Backlog", ProjectID: project.ID, Default: true}))
			}
			column := &project_model.Column{Title: "Done", ProjectID: project.ID}
			require.NoError(t, project_model.NewColumn(ctx, column))
			issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 6})
			_, _, err := issues_model.IssueAssignOrRemoveProject(ctx, issue, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}), []int64{project.ID})
			require.NoError(t, err)
			ctx.SetPathParam("id", strconv.FormatInt(project.ID, 10))
			ctx.SetPathParam("columnID", strconv.FormatInt(column.ID, 10))
			ctx.Req.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"issues":[{"issueID":%d,"sorting":0}]}`, issue.ID)))

			MoveIssues(ctx)
			assert.Equal(t, tc.status, ctx.Resp.WrittenStatus())
			card := unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: issue.ID, ProjectID: project.ID})
			assert.Equal(t, tc.status == http.StatusOK, card.ProjectColumnID == column.ID)
		})
	}
}
