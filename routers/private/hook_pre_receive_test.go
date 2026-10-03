// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"
	"testing"

	actions_model "gitea.dev/models/actions"
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
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookPreReceiveActionsProtectedBranch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.NoError(t, db.Insert(t.Context(), &repo_model.RepoUnit{
		RepoID: repo.ID,
		Type:   unit.TypeActions,
		Config: &repo_model.ActionsConfig{},
	}))
	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 53})
	require.Equal(t, repo.ID, task.RepoID)
	require.NoError(t, task.LoadJob(t.Context()))
	doer := user_model.NewActionsUserWithTaskID(task.ID)

	head := "1032bbf17fbc0d9c95bb5418dabe8f8c99278700"
	parent := "205ac761f3326a7ebe416e8673760016450b5cec"
	empty := git.ObjectFormatFromName(repo.ObjectFormatName).EmptyObjectID().String()
	for _, tc := range []struct {
		name       string
		protection git_model.ProtectedBranch
		readOnly   bool
		forcePush  bool
		delete     bool
		wantError  string
	}{
		{name: "write token", protection: git_model.ProtectedBranch{CanPush: true}},
		{name: "read token", protection: git_model.ProtectedBranch{CanPush: true}, readOnly: true, wantError: "User permission denied for writing"},
		{name: "push disabled", wantError: "Not allowed to push to protected branch"},
		{name: "push allowlist", protection: git_model.ProtectedBranch{CanPush: true, EnableWhitelist: true}, wantError: "Not allowed to push to protected branch"},
		{name: "force push", protection: git_model.ProtectedBranch{CanPush: true, CanForcePush: true}, forcePush: true},
		{name: "force push disabled", protection: git_model.ProtectedBranch{CanPush: true}, forcePush: true, wantError: "protected from force push"},
		{name: "force push with push disabled", protection: git_model.ProtectedBranch{CanForcePush: true}, forcePush: true, wantError: "Not allowed to force-push to protected branch"},
		{name: "force push with push allowlist", protection: git_model.ProtectedBranch{CanPush: true, CanForcePush: true, EnableWhitelist: true}, forcePush: true, wantError: "Not allowed to force-push to protected branch"},
		{name: "force push allowlist", protection: git_model.ProtectedBranch{CanPush: true, CanForcePush: true, EnableForcePushAllowlist: true}, forcePush: true, wantError: "Not allowed to force-push to protected branch"},
		{name: "protected files", protection: git_model.ProtectedBranch{CanPush: true, ProtectedFilePatterns: "test.xml"}, wantError: "protected from changing file"},
		{name: "delete branch", protection: git_model.ProtectedBranch{CanPush: true}, delete: true, wantError: "protected from deletion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			protection := tc.protection
			protection.RepoID = repo.ID
			protection.RuleName = "probe"
			require.NoError(t, db.Insert(t.Context(), &protection))
			defer func() {
				require.NoError(t, git_model.DeleteProtectedBranch(t.Context(), repo, protection.ID))
			}()

			mode := perm.AccessModeWrite
			if tc.readOnly {
				mode = perm.AccessModeRead
			}
			task.Job.TokenPermissions = &repo_model.ActionsTokenPermissions{UnitAccessModes: map[unit.Type]perm.AccessMode{unit.TypeCode: mode}}
			_, err := db.GetEngine(t.Context()).ID(task.JobID).Cols("token_permissions").Update(task.Job)
			require.NoError(t, err)

			ctx, resp := contexttest.MockPrivateContext(t, "/")
			ctx.SetPathParam("owner", "user2")
			ctx.SetPathParam("repo", repo.Name)
			RepoAssignment(ctx)
			require.False(t, ctx.Written())
			t.Cleanup(func() { ctx.Repo.GitRepo.Close() })
			oldCommitID, newCommitID := parent, head
			if tc.forcePush {
				oldCommitID, newCommitID = head, parent
			} else if tc.delete {
				newCommitID = empty
			}
			web.SetForm(ctx, &private.HookOptions{
				UserID:          doer.ID,
				UserExtDoerData: doer.ExtDoerData.EncodeToString(),
				OldCommitIDs:    []string{oldCommitID},
				NewCommitIDs:    []string{newCommitID},
				RefFullNames:    []git.RefName{git.RefNameFromBranch("probe")},
			})
			HookPreReceive(ctx)
			if tc.wantError == "" {
				assert.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
				assert.Equal(t, "ok", resp.Body.String())
			} else {
				assert.Equal(t, http.StatusForbidden, resp.Code, resp.Body.String())
				assert.Contains(t, resp.Body.String(), tc.wantError)
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
