// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestDeleteColumn_MovesIssuesToDefault(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Project 1 fixtures: column 1 is default (has issue 1), column 2 has issue 3,
	// column 3 has issue 5. Delete column 2; its issue should land on column 1.
	col2 := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 2})

	err := DeleteColumn(t.Context(), col2)
	assert.NoError(t, err)

	// Column row is gone.
	unittest.AssertNotExistsBean(t, &project_model.Column{ID: 2})

	// Issue 3 is now on column 1 at the new max+1.
	pi := unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: 3, ProjectID: 1})
	assert.EqualValues(t, 1, pi.ProjectColumnID)
	// Column 1 already had issue 1 at sorting 0, so the moved issue lands at sorting 1.
	assert.EqualValues(t, 1, pi.Sorting)
}

func TestDeleteColumn_RefusesDefault(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	col1 := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 1})
	assert.True(t, col1.Default)

	err := DeleteColumn(t.Context(), col1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default")

	// Column and its issue are still there.
	unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: 1, ProjectID: 1})

	_ = issues_model.Issue{} // keep the import
}

func TestReorderColumns_MovesAll(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project1 := unittest.AssertExistsAndLoadBean(t, &project_model.Project{ID: 1})
	columns, err := project1.GetColumns(t.Context())
	assert.NoError(t, err)
	assert.Len(t, columns, 3)

	err = ReorderColumns(t.Context(), project1, map[int64]int64{
		columns[1].ID: 0,
		columns[2].ID: 1,
		columns[0].ID: 2,
	})
	assert.NoError(t, err)

	after, err := project1.GetColumns(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, columns[1].ID, after[0].ID)
	assert.Equal(t, columns[2].ID, after[1].ID)
	assert.Equal(t, columns[0].ID, after[2].ID)
}
