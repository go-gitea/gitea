// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"net/url"
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"
	mirror_service "gitea.dev/services/mirror"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddDeployKey(t *testing.T) {
	unittest.PrepareTestEnv(t)
	t.Run("ReadOnly", func(t *testing.T) {
		const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf\n"
		ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/keys")
		contexttest.MockRequestPostForm(ctx.Req, url.Values{"title": {"read-only"}, "content": {testKey}})
		contexttest.LoadRepo(t, ctx, 2)
		DeployKeysPost(ctx)
		assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
		unittest.AssertExistsAndLoadBean(t, &asymkey_model.DeployKey{Name: "read-only", Mode: perm.AccessModeRead})
	})
	t.Run("ReadWrite", func(t *testing.T) {
		const testKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEHjnNEfE88W1pvBLdV3otv28x760gdmPao3lVD5uAt9\n"
		ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/keys")
		contexttest.MockRequestPostForm(ctx.Req, url.Values{"title": {"read-write"}, "content": {testKey}, "is_writable": {"on"}})
		contexttest.LoadRepo(t, ctx, 2)
		DeployKeysPost(ctx)
		assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
		unittest.AssertExistsAndLoadBean(t, &asymkey_model.DeployKey{Name: "read-write", Mode: perm.AccessModeWrite})
	})
}

func TestCollaborationPost(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1/issues/labels")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadUser(t, ctx, 4)
	contexttest.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user4")

	u := &user_model.User{
		LowerName: "user2",
		Type:      user_model.UserTypeIndividual,
	}

	re := &repo_model.Repository{
		ID:    2,
		Owner: u,
	}

	repo := &context.Repository{
		Owner:      u,
		Repository: re,
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

	exists, err := repo_model.IsCollaborator(ctx, re.ID, 4)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestCollaborationPost_InactiveUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1/issues/labels")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadUser(t, ctx, 9)
	contexttest.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user9")

	repo := &context.Repository{
		Owner: &user_model.User{
			LowerName: "user2",
		},
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestCollaborationPost_AddCollaboratorTwice(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1/issues/labels")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadUser(t, ctx, 4)
	contexttest.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user4")

	u := &user_model.User{
		LowerName: "user2",
		Type:      user_model.UserTypeIndividual,
	}

	re := &repo_model.Repository{
		ID:    2,
		Owner: u,
	}

	repo := &context.Repository{
		Owner:      u,
		Repository: re,
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

	exists, err := repo_model.IsCollaborator(ctx, re.ID, 4)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Try adding the same collaborator again
	CollaborationPost(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestCollaborationPost_NonExistentUser(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1/issues/labels")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user34")

	repo := &context.Repository{
		Owner: &user_model.User{
			LowerName: "user2",
		},
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost(t *testing.T) {
	unittest.PrepareTestEnv(t)
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 26})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 43})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 11})
	repo.Owner = org

	testAddTeamPost := func(t *testing.T, teamName string, repoAdminChangeTeamAccess bool) *context.Context {
		ctx, _ := contexttest.MockContext(t, "org26/repo43")
		ctx.Req.Form.Set("team", teamName)
		org.RepoAdminChangeTeamAccess = repoAdminChangeTeamAccess
		ctx.Repo = &context.Repository{
			Permission: access_model.Permission{AccessMode: perm.AccessModeAdmin},
			Owner:      repo.Owner,
			Repository: repo,
		}
		ctx.Doer = &user_model.User{ID: 1, IsAdmin: true}
		AddTeamPost(ctx)
		return ctx
	}

	t.Run("NonExisting", func(t *testing.T) {
		ctx := testAddTeamPost(t, "team-not-exist", true)
		assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
		assert.Contains(t, ctx.Flash.ErrorMsg, "form.team_not_exist")
	})
	t.Run("NotAllowed", func(t *testing.T) {
		ctx := testAddTeamPost(t, team.Name, false)
		assert.False(t, repo_service.HasRepository(t.Context(), team, repo.ID))
		assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
		assert.Contains(t, ctx.Flash.ErrorMsg, "repo.settings.change_team_access_not_allowed")
	})
	t.Run("Allowed", func(t *testing.T) {
		ctx := testAddTeamPost(t, team.Name, true)
		assert.True(t, repo_service.HasRepository(t.Context(), team, repo.ID))
		assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
		assert.Empty(t, ctx.Flash.ErrorMsg)
		t.Run("Twice", func(t *testing.T) {
			ctx := testAddTeamPost(t, team.Name, true)
			assert.True(t, repo_service.HasRepository(t.Context(), team, repo.ID))
			assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
			assert.Contains(t, ctx.Flash.ErrorMsg, "repo.settings.add_team_duplicate")
		})
	})
}

func TestDeleteTeam(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "org3/team1/repo3")

	ctx.Req.Form.Set("id", "2")
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	repo.Owner = org
	org.RepoAdminChangeTeamAccess = true
	ctx.Repo = &context.Repository{
		Permission: access_model.Permission{AccessMode: perm.AccessModeAdmin},
		Owner:      repo.Owner,
		Repository: repo,
	}
	ctx.Doer = &user_model.User{ID: 1, IsAdmin: true}

	assert.True(t, repo_service.HasRepository(t.Context(), team, repo.ID))
	DeleteTeam(ctx)
	assert.False(t, repo_service.HasRepository(t.Context(), team, repo.ID))
}

func TestHandleSettingsPostMirrorPreservesExistingUsername(t *testing.T) {
	defer test.MockVariableValue(&setting.Mirror.Enabled, true)()

	unittest.PrepareTestEnv(t)

	// Use the existing fixture mirror repo (org3/repo5) which has a git repo on disk.
	mirrorRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
	mirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: 5})

	require.NoError(t, mirror_service.UpdateAddress(t.Context(), mirror, "https://existing-user:existing-password@example.com/user2/repo1.git"))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	ctx, _ := contexttest.MockContext(t, mirrorRepo.Link()+"/settings")
	contexttest.LoadUser(t, ctx, user.ID)
	contexttest.LoadRepo(t, ctx, mirrorRepo.ID)

	web.SetForm(ctx, &forms.RepoSettingForm{
		Interval:       "8h",
		MirrorAddress:  "https://example.com/user2/repo1.git",
		MirrorPassword: "updated-password",
	})

	handleSettingsPostMirror(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

	updatedMirror := unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: mirrorRepo.ID})
	assert.Equal(t, "https://example.com/user2/repo1.git", updatedMirror.RemoteAddress)

	updatedRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: mirrorRepo.ID})
	assert.Equal(t, "https://example.com/user2/repo1.git", updatedRepo.OriginalURL)

	remoteURL, err := git.ParseRemoteAddressURL(t.Context(), updatedRepo, updatedMirror.GetRemoteName())
	require.NoError(t, err)
	require.NotNil(t, remoteURL.User)
	assert.Equal(t, "existing-user", remoteURL.User.Username())
	password, ok := remoteURL.User.Password()
	require.True(t, ok)
	assert.Equal(t, "updated-password", password)
}
