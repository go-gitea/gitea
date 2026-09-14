// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	notify_service "gitea.dev/services/notify"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type reviewContentNotifier struct {
	notify_service.NullNotifier
	inTransaction []bool
}

func (n *reviewContentNotifier) UpdateComment(ctx context.Context, _ *user_model.User, _ *issues_model.Comment, _ string) {
	n.inTransaction = append(n.inTransaction, db.InTransaction(ctx))
}

func TestUpdateReviewContentNotifications(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	notifier := &reviewContentNotifier{}
	notify_service.RegisterNotifier(notifier)
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 10})
	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: comment.ReviewID})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: comment.PosterID})
	require.NoError(t, UpdateReviewContent(t.Context(), review, doer, "updated summary"))
	assert.Equal(t, []bool{false}, notifier.inTransaction)
	assert.Equal(t, review.Content, unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: comment.ID}).Content)

	pending := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 4})
	require.NoError(t, db.Insert(t.Context(), &issues_model.Comment{
		Type: issues_model.CommentTypeReview, IssueID: pending.IssueID, ReviewID: pending.ID,
		PosterID: pending.ReviewerID, Content: "draft summary",
	}))
	require.NoError(t, UpdateReviewContent(t.Context(), pending, doer, "private summary"))
	assert.Equal(t, []bool{false}, notifier.inTransaction)
}
