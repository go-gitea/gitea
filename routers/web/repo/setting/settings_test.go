// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"testing"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func createSSHAuthorizedKeysTmpPath(t *testing.T) func() {
	tmpDir := t.TempDir()

	oldPath := setting.SSH.RootPath
	setting.SSH.RootPath = tmpDir

	return func() {
		setting.SSH.RootPath = oldPath
	}
}

func TestAddReadOnlyDeployKey(t *testing.T) {
	if deferable := createSSHAuthorizedKeysTmpPath(t); deferable != nil {
		defer deferable()
	} else {
		return
	}
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockContext(t, "user2/repo1/settings/keys")

	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 2)

	addKeyForm := forms.AddKeyForm{
		Title:   "read-only",
		Content: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
	}
	web.SetForm(ctx, &addKeyForm)
	DeployKeysPost(ctx)
	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

	unittest.AssertExistsAndLoadBean(t, &asymkey_model.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    perm.AccessModeRead,
	})
}

func TestAddReadWriteOnlyDeployKey(t *testing.T) {
	if deferable := createSSHAuthorizedKeysTmpPath(t); deferable != nil {
		defer deferable()
	} else {
		return
	}

	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockContext(t, "user2/repo1/settings/keys")

	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 2)

	addKeyForm := forms.AddKeyForm{
		Title:      "read-write",
		Content:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		IsWritable: true,
	}
	web.SetForm(ctx, &addKeyForm)
	DeployKeysPost(ctx)
	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

	unittest.AssertExistsAndLoadBean(t, &asymkey_model.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    perm.AccessModeWrite,
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

	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

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

	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
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

	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())

	exists, err := repo_model.IsCollaborator(ctx, re.ID, 4)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Try adding the same collaborator again
	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
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

	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
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

	assert.True(t, repo_service.HasRepository(db.DefaultContext, team, re.ID))
	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.Empty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost_NotAllowed(t *testing.T) {
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
			RepoAdminChangeTeamAccess: false,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)

	assert.False(t, repo_service.HasRepository(db.DefaultContext, team, re.ID))
	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
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
	assert.True(t, repo_service.HasRepository(db.DefaultContext, team, re.ID))
	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
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
	assert.EqualValues(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
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

	assert.False(t, repo_service.HasRepository(db.DefaultContext, team, re.ID))
}
