// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"net/http"
	"testing"

	"codenest.dev/models/db"
	issues_model "codenest.dev/models/issues"
	"codenest.dev/models/organization"
	"codenest.dev/models/perm"
	project_model "codenest.dev/models/project"
	"codenest.dev/models/unit"
	"codenest.dev/models/unittest"
	user_model "codenest.dev/models/user"
	"codenest.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateIssueProjectColumnNeedsItemRead(t *testing.T) {
	unittest.PrepareTestEnv(t)
	_, err := db.Exec(t.Context(), "UPDATE team SET authorize = ? WHERE id = ?", perm.AccessModeNone, 2)
	require.NoError(t, err)
	_, err = db.DeleteByBean(t.Context(), &organization.TeamUnit{TeamID: 2, Type: unit.TypePullRequests})
	require.NoError(t, err)
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 12})
	_, _, err = issues_model.IssueAssignOrRemoveProject(t.Context(), pull, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}), []int64{2})
	require.NoError(t, err)
	column := &project_model.Column{Title: "Done", ProjectID: 2}
	require.NoError(t, project_model.NewColumn(t.Context(), column))

	ctx, _ := contexttest.MockContext(t, fmt.Sprintf("POST /org3/repo3/issues/projects/column?issue_id=%d&id=%d", pull.ID, column.ID))
	contexttest.LoadUser(t, ctx, 4)
	contexttest.LoadRepo(t, ctx, 3)
	UpdateIssueProjectColumn(ctx)
	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
	assert.NotEqual(t, column.ID, unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: pull.ID, ProjectID: 2}).ProjectColumnID)
}
