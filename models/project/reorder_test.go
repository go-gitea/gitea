// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_SetColumnSortings_Swap(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Project 1 fixture: columns id=1 sorting=0, id=2 sorting=1, id=3 sorting=2.
	// Swap 2 and 3: id=2 → 2, id=3 → 1.
	err := SetColumnSortings(t.Context(), 1, map[int64]int64{
		2: 2,
		3: 1,
	})
	assert.NoError(t, err)

	col2 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 2})
	col3 := unittest.AssertExistsAndLoadBean(t, &Column{ID: 3})
	assert.EqualValues(t, 2, col2.Sorting)
	assert.EqualValues(t, 1, col3.Sorting)
}

func Test_SetIssueSortingsInColumn_SwapWithinColumn(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Put issue 4 onto project 1 / column 2 at sorting 1 (existing issue 3 is at sorting 0).
	_, err := AppendIssueToColumn(t.Context(), 1, 2, 4)
	assert.NoError(t, err)

	// Swap issue 3 and issue 4 sortings within column 2.
	err = SetIssueSortingsInColumn(t.Context(), 2, map[int64]int64{
		3: 1,
		4: 0,
	})
	assert.NoError(t, err)

	pi3 := unittest.AssertExistsAndLoadBean(t, &ProjectIssue{IssueID: 3, ProjectID: 1})
	pi4 := unittest.AssertExistsAndLoadBean(t, &ProjectIssue{IssueID: 4, ProjectID: 1})
	assert.EqualValues(t, 1, pi3.Sorting)
	assert.EqualValues(t, 0, pi4.Sorting)
	assert.EqualValues(t, 2, pi3.ProjectColumnID)
	assert.EqualValues(t, 2, pi4.ProjectColumnID)
}

func Test_AppendIssueToColumn_ComputesMaxPlusOne(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Issue 3 is already on project 1 / column 2 at sorting 0.
	sorting, err := AppendIssueToColumn(t.Context(), 1, 2, 4)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, sorting)

	pi := unittest.AssertExistsAndLoadBean(t, &ProjectIssue{IssueID: 4, ProjectID: 1})
	assert.EqualValues(t, 2, pi.ProjectColumnID)
	assert.EqualValues(t, 1, pi.Sorting)
}

func Test_AppendIssueToColumn_EmptyColumnStartsAtZero(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Project 1 / column 3 has one issue (id=5, sorting=0 from fixtures).
	// Use project 5 / column 7 which has no issues.
	sorting, err := AppendIssueToColumn(t.Context(), 5, 7, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, sorting)
}
