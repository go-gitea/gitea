// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPreReceiveCanWriteCodePerBranch ensures the maintainer-edit write grant is evaluated against
// the current branch on every call, instead of being cached from the first ref of a batch push.
// Otherwise a per-branch grant (an open PR with "allow edits from maintainers") could be batched
// together with a protected branch to escalate into full repository write.
func TestPreReceiveCanWriteCodePerBranch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	headRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	require.NoError(t, baseRepo.LoadOwner(t.Context()))
	require.NoError(t, headRepo.LoadOwner(t.Context()))

	// An open PR from the head repo owner, with maintainer edits allowed: this grants the base
	// repo owner write access to exactly this head branch and nothing else.
	pr := &issues_model.PullRequest{
		Issue: &issues_model.Issue{
			RepoID:   baseRepo.ID,
			PosterID: headRepo.OwnerID,
		},
		HeadRepoID:          headRepo.ID,
		BaseRepoID:          baseRepo.ID,
		HeadBranch:          "granted-branch",
		BaseBranch:          "master",
		AllowMaintainerEdit: true,
	}
	require.NoError(t, issues_model.NewPullRequest(t.Context(), baseRepo, pr.Issue, nil, nil, pr))

	// The pusher is the base repo owner (the maintainer) with only read access on the head repo.
	maintainer := baseRepo.Owner
	headPerm, err := access.GetIndividualUserRepoPermission(t.Context(), headRepo, maintainer)
	require.NoError(t, err)

	mockCtx, _ := contexttest.MockPrivateContext(t, "/")
	ctx := &preReceiveContext{
		PrivateContext: mockCtx,
		loadedPusher:   true,
		user:           maintainer,
		userPerm:       headPerm,
	}

	// The granted branch must be writable...
	ctx.branchName = "granted-branch"
	assert.True(t, ctx.CanWriteCode())

	// ...but another branch in the same push must NOT inherit that grant.
	ctx.branchName = "master"
	assert.False(t, ctx.CanWriteCode())
}
