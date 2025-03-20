// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func Test_ToPullReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	review := unittest.AssertExistsAndLoadBean(t, &issues_model.Review{ID: 6})
	assert.EqualValues(t, reviewer.ID, review.ReviewerID)
	assert.EqualValues(t, issues_model.ReviewTypePending, review.Type)

	reviewList := []*issues_model.Review{review}

	t.Run("Anonymous User", func(t *testing.T) {
		prList, err := ToPullReviewList(db.DefaultContext, reviewList, nil)
		assert.NoError(t, err)
		assert.Empty(t, prList)
	})

	t.Run("Reviewer Himself", func(t *testing.T) {
		prList, err := ToPullReviewList(db.DefaultContext, reviewList, reviewer)
		assert.NoError(t, err)
		assert.Len(t, prList, 1)
	})

	t.Run("Other User", func(t *testing.T) {
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		prList, err := ToPullReviewList(db.DefaultContext, reviewList, user4)
		assert.NoError(t, err)
		assert.Empty(t, prList)
	})

	t.Run("Admin User", func(t *testing.T) {
		adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		prList, err := ToPullReviewList(db.DefaultContext, reviewList, adminUser)
		assert.NoError(t, err)
		assert.Len(t, prList, 1)
	})
}
