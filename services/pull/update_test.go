// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestIsUserAllowedToUpdateRespectsProtectedBranch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	protectedBranch := &git_model.ProtectedBranch{
		RepoID:       pr.HeadRepoID,
		RuleName:     pr.HeadBranch,
		CanPush:      false,
		CanForcePush: false,
	}
	_, err := db.GetEngine(t.Context()).Insert(protectedBranch)
	assert.NoError(t, err)

	pushAllowed, rebaseAllowed, err := IsUserAllowedToUpdate(t.Context(), pr, user)
	assert.NoError(t, err)
	assert.False(t, pushAllowed)
	assert.False(t, rebaseAllowed)
}
