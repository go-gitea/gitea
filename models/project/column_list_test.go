// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetColumnsPaginated(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const projectID = 1
	count, err := CountColumns(t.Context(), projectID)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)

	// Page 1, limit 2 — returns first 2 columns
	page1, err := GetColumns(t.Context(), projectID, db.ListOptions{Page: 1, PageSize: 2})
	assert.NoError(t, err)
	assert.Len(t, page1, 2)

	// Page 2, limit 2 — returns remaining column
	page2, err := GetColumns(t.Context(), projectID, db.ListOptions{Page: 2, PageSize: 2})
	assert.NoError(t, err)
	assert.Len(t, page2, 1)

	// Page 1 and page 2 together cover all columns with no overlap
	allIDs := make(map[int64]bool)
	for _, c := range append(page1, page2...) {
		assert.False(t, allIDs[c.ID], "duplicate column ID %d across pages", c.ID)
		allIDs[c.ID] = true
	}
	assert.Len(t, allIDs, 3)
}
