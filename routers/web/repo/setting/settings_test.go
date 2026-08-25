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
	ctx, _ := contexttest.MockContext(t, "org26/repo43")

	ctx.Req.Form.Set("team", "team11")

	org := &user_model.User{
		LowerName: "org26",
		Type:      user_model.UserTypeOrganization,
	}

	team := &organization.Team{
		ID:    11,
		OrgID: 26,
	}

	re := &repo_model.Repository{
		ID:      43,
		Owner:   org,
		OwnerID: 26,
	}

	repo := &context.Repository{
		Owner: &user_model.User{
			ID:                        26,
			LowerName:                 "org26",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)

	assert.True(t, repo_service.HasRepository(t.Context(), team, re.ID))
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.Empty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost_NotAllowed(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32})
	require.NoError(t, repo.LoadOwner(t.Context()))
	adminTeam := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 12})
	targetTeam := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 2})
	require.NoError(t, repo_service.TeamAddRepository(t.Context(), adminTeam, repo))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	repoContext := &context.Repository{Owner: repo.Owner, Repository: repo}
	renderCtx, _ := contexttest.MockContext(t, repo.Link()+"/settings/collaboration")
	renderCtx.Repo = repoContext
	renderCtx.Doer = doer
	Collaboration(renderCtx)
	assert.Equal(t, false, renderCtx.Data["CanChangeRepoTeamAccess"])

	ctx, _ := contexttest.MockContext(t, repo.Link()+"/settings/collaboration")
	ctx.Req.Form.Set("team", targetTeam.Name)
	ctx.Repo = repoContext
	ctx.Doer = doer

	AddTeamPost(ctx)

	assert.False(t, repo_service.HasRepository(t.Context(), targetTeam, repo.ID))
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost_AddTeamTwice(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "org26/repo43")

	ctx.Req.Form.Set("team", "team11")

	org := &user_model.User{
		LowerName: "org26",
		Type:      user_model.UserTypeOrganization,
	}

	team := &organization.Team{
		ID:    11,
		OrgID: 26,
	}

	re := &repo_model.Repository{
		ID:      43,
		Owner:   org,
		OwnerID: 26,
	}

	repo := &context.Repository{
		Owner: &user_model.User{
			ID:                        26,
			LowerName:                 "org26",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)

	AddTeamPost(ctx)
	assert.True(t, repo_service.HasRepository(t.Context(), team, re.ID))
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost_NonExistentTeam(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "org26/repo43")

	ctx.Req.Form.Set("team", "team-non-existent")

	org := &user_model.User{
		LowerName: "org26",
		Type:      user_model.UserTypeOrganization,
	}

	re := &repo_model.Repository{
		ID:      43,
		Owner:   org,
		OwnerID: 26,
	}

	repo := &context.Repository{
		Owner: &user_model.User{
			ID:                        26,
			LowerName:                 "org26",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestDeleteTeam(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "org3/team1/repo3")

	ctx.Req.Form.Set("id", "2")

	org := &user_model.User{
		LowerName: "org3",
		Type:      user_model.UserTypeOrganization,
	}

	team := &organization.Team{
		ID:    2,
		OrgID: 3,
	}

	re := &repo_model.Repository{
		ID:      3,
		Owner:   org,
		OwnerID: 3,
	}

	repo := &context.Repository{
		Owner: &user_model.User{
			ID:                        3,
			LowerName:                 "org3",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	DeleteTeam(ctx)

	assert.False(t, repo_service.HasRepository(t.Context(), team, re.ID))
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
