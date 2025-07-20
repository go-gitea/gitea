// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func Test_DeleteCommentWithReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 7})
	assert.NoError(t, comment.LoadAttachments(t.Context()))
	assert.Len(t, comment.Attachments, 1)
	assert.Equal(t, int64(13), comment.Attachments[0].ID)
	assert.Equal(t, int64(10), comment.ReviewID)
	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: comment.ReviewID})
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// since this is the last comment of the review, it should be deleted when the comment is deleted
	deletedReviewComment, err := DeleteComment(db.DefaultContext, user1, comment)
	assert.NoError(t, err)
	assert.NotNil(t, deletedReviewComment)

	// the review should be deleted as well
	unittest.AssertNotExistsBean(t, &issues_model.Review{ID: review.ID})
	// the attachment should be deleted as well
	newAttachment, err := repo_model.GetAttachmentByID(t.Context(), comment.Attachments[0].ID)
	assert.Error(t, err)
	assert.Nil(t, newAttachment)
}
