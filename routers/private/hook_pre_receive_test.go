// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/private"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreReceiveActionsProtectedBranch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	for _, tc := range []struct {
		name       string
		protection git_model.ProtectedBranch
		forcePush  bool
		allowed    bool
	}{
		{name: "push", protection: git_model.ProtectedBranch{CanPush: true}, allowed: true},
		{name: "push disabled"},
		{name: "push allowlist", protection: git_model.ProtectedBranch{CanPush: true, EnableWhitelist: true}},
		{name: "force push", protection: git_model.ProtectedBranch{CanPush: true, CanForcePush: true}, forcePush: true, allowed: true},
		{name: "force push with push allowlist", protection: git_model.ProtectedBranch{CanPush: true, CanForcePush: true, EnableWhitelist: true}, forcePush: true},
		{name: "force push allowlist", protection: git_model.ProtectedBranch{CanPush: true, CanForcePush: true, EnableForcePushAllowlist: true}, forcePush: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockCtx, resp := contexttest.MockPrivateContext(t, "/")
			ctx := &preReceiveContext{PrivateContext: mockCtx, opts: &private.HookOptions{UserID: user_model.ActionsUserID}}
			ctx.SetPathParam("owner", "user2")
			ctx.SetPathParam("repo", "repo2")
			RepoAssignment(ctx.PrivateContext)
			require.False(t, ctx.Written())
			defer ctx.Repo.GitRepo.Close()
			ctx.Doer = user_model.NewActionsUserWithTaskID(53)
			ctx.Repo.Permission.SetUnitsWithDefaultAccessMode([]*repo_model.RepoUnit{{Type: unit.TypeCode}}, perm.AccessModeWrite)

			protection := tc.protection
			protection.RepoID = ctx.Repo.Repository.ID
			protection.RuleName = "probe"
			require.NoError(t, db.Insert(t.Context(), &protection))
			defer func() {
				require.NoError(t, git_model.DeleteProtectedBranch(t.Context(), ctx.Repo.Repository, protection.ID))
			}()

			oldCommitID, newCommitID := "205ac761f3326a7ebe416e8673760016450b5cec", "1032bbf17fbc0d9c95bb5418dabe8f8c99278700"
			if tc.forcePush {
				oldCommitID, newCommitID = newCommitID, oldCommitID
			}
			preReceiveBranch(ctx, oldCommitID, newCommitID, git.RefNameFromBranch("probe"))
			assert.Equal(t, !tc.allowed, ctx.Written(), resp.Body.String())
			if !tc.allowed {
				assert.Equal(t, http.StatusForbidden, resp.Code)
				assert.Contains(t, resp.Body.String(), "Not allowed to")
			}
		})
	}
}

// TestPreReceiveCanWriteCodePerBranch ensures the maintainer-edit write grant is evaluated against
// the exact ref being pushed on every call, derived from that ref rather than shared mutable state.
// Otherwise, a per-branch grant (an open PR with "allow edits from maintainers") could be batched
// together with a protected branch or a tag to escalate into full repository write.
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
	mockCtx, _ := contexttest.MockPrivateContext(t, "/")
	ctx := &preReceiveContext{PrivateContext: mockCtx}
	ctx.SetPathParam("owner", headRepo.OwnerName)
	ctx.SetPathParam("repo", headRepo.Name)
	RepoAssignment(ctx.PrivateContext)
	loadContextDoerPermission(ctx.PrivateContext, baseRepo.OwnerID, "")

	// The granted branch must be writable...
	assert.True(t, ctx.canWriteCodeRef(git.RefNameFromBranch("granted-branch")))

	// ...but another branch in the same push must NOT inherit that grant.
	assert.False(t, ctx.canWriteCodeRef(git.RefNameFromBranch("master")))

	// ...and a tag sharing the granted branch's name must NOT inherit it either: the grant is
	// scoped to PR head branches, so a non-branch ref can never match it. (A tag ref already
	// yields an empty branch name, so this guards the per-ref evaluation, not the IsBranch check.)
	assert.False(t, ctx.canWriteCodeRef(git.RefNameFromTag("granted-branch")))
}
