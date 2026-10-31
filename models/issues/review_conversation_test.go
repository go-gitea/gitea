// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetConversationStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const issueID = 9000
	add := func(c *issues_model.Comment) *issues_model.Comment {
		c.IssueID = issueID
		c.Type = issues_model.CommentTypeCode
		_, err := db.GetEngine(t.Context()).Insert(c)
		assert.NoError(t, err)
		return c
	}
	pending := &issues_model.Review{IssueID: issueID, Type: issues_model.ReviewTypePending, ReviewerID: 1}
	_, err := db.GetEngine(t.Context()).Insert(pending)
	assert.NoError(t, err)

	resolvedRoot := add(&issues_model.Comment{TreePath: "a.go", Line: 1, ResolveDoerID: 2})
	add(&issues_model.Comment{TreePath: "a.go", Line: 1}) // reply, not counted
	openRoot := add(&issues_model.Comment{TreePath: "b.go", Line: 3})
	add(&issues_model.Comment{TreePath: "c.go", Line: 4, Invalidated: true}) // outdated conversations still count
	add(&issues_model.Comment{TreePath: "c.go", Line: 4})                    // new conversation on a line that has an outdated one
	add(&issues_model.Comment{TreePath: "e.go", Line: -6, Invalidated: true})
	add(&issues_model.Comment{TreePath: "e.go", Line: -6}) // reply on the old side does not inherit the outdated flag
	add(&issues_model.Comment{TreePath: "d.go", Line: 5, ReviewID: pending.ID})

	stats, err := issues_model.GetConversationStats(t.Context(), issueID)
	assert.NoError(t, err)
	assert.Equal(t, 5, stats.Total)
	assert.Equal(t, 1, stats.Resolved)
	assert.Equal(t, 4, stats.Unresolved())
	assert.Equal(t, openRoot.ID, stats.NextOpenID)
	assert.NotEqual(t, resolvedRoot.ID, stats.NextOpenID)

	stats, err = issues_model.GetConversationStats(t.Context(), issueID+1)
	assert.NoError(t, err)
	assert.Equal(t, 0, stats.Total)
}
