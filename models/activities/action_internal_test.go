// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"fmt"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func TestHiddenConfidentialActionsCondSize(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	viewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})

	condSQL := func(opts GetFeedsOptions, requestedRepoID int64) string {
		feedCond, err := ActivityQueryCondition(ctx, opts)
		require.NoError(t, err)
		cond, err := hiddenConfidentialActionsCond(ctx, viewer, feedCond, requestedRepoID)
		require.NoError(t, err)
		sql, args, err := builder.ToSQL(cond)
		require.NoError(t, err)
		return fmt.Sprint(sql, args)
	}
	userFeed := GetFeedsOptions{RequestedUser: owner, Actor: viewer, IncludePrivate: true}
	repoFeed := GetFeedsOptions{RequestedRepo: repo, Actor: viewer, IncludePrivate: true}
	userBefore, repoBefore := condSQL(userFeed, 0), condSQL(repoFeed, repo.ID)

	for index := int64(9100); index < 9150; index++ {
		require.NoError(t, db.Insert(ctx, &issues_model.Issue{RepoID: repo.ID, Repo: repo, Index: index, PosterID: 2, Title: "t", IsConfidential: true}))
	}
	assert.Equal(t, userBefore, condSQL(userFeed, 0))
	assert.Equal(t, repoBefore, condSQL(repoFeed, repo.ID))
}
