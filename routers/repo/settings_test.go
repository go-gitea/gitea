// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func createSSHAuthorizedKeysTmpPath(t *testing.T) func() {
	tmpDir, err := ioutil.TempDir("", "tmp-ssh")
	if err != nil {
		assert.Fail(t, "Unable to create temporary directory: %v", err)
		return nil
	}

	oldPath := setting.SSH.RootPath
	setting.SSH.RootPath = tmpDir

	return func() {
		setting.SSH.RootPath = oldPath
		os.RemoveAll(tmpDir)
	}
}

func TestAddReadOnlyDeployKey(t *testing.T) {
	if deferable := createSSHAuthorizedKeysTmpPath(t); deferable != nil {
		defer deferable()
	} else {
		return
	}
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/settings/keys")

	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 2)

	addKeyForm := auth.AddKeyForm{
		Title:   "read-only",
		Content: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
	}
	DeployKeysPost(ctx, addKeyForm)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    models.AccessModeRead,
	})
}

func TestAddReadWriteOnlyDeployKey(t *testing.T) {
	if deferable := createSSHAuthorizedKeysTmpPath(t); deferable != nil {
		defer deferable()
	} else {
		return
	}

	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/settings/keys")

	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 2)

	addKeyForm := auth.AddKeyForm{
		Title:      "read-write",
		Content:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n",
		IsWritable: true,
	}
	DeployKeysPost(ctx, addKeyForm)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.DeployKey{
		Name:    addKeyForm.Title,
		Content: addKeyForm.Content,
		Mode:    models.AccessModeWrite,
	})
}

func TestCollaborationPost(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadUser(t, ctx, 4)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user4")

	u := &models.User{
		LowerName: "user2",
		Type:      models.UserTypeIndividual,
	}

	re := &models.Repository{
		ID:    2,
		Owner: u,
	}

	repo := &context.Repository{
		Owner:      u,
		Repository: re,
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	exists, err := re.IsCollaborator(4)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestCollaborationPost_InactiveUser(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadUser(t, ctx, 9)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user9")

	repo := &context.Repository{
		Owner: &models.User{
			LowerName: "user2",
		},
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestCollaborationPost_AddCollaboratorTwice(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadUser(t, ctx, 4)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user4")

	u := &models.User{
		LowerName: "user2",
		Type:      models.UserTypeIndividual,
	}

	re := &models.Repository{
		ID:    2,
		Owner: u,
	}

	repo := &context.Repository{
		Owner:      u,
		Repository: re,
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())

	exists, err := re.IsCollaborator(4)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Try adding the same collaborator again
	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestCollaborationPost_NonExistentUser(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1/issues/labels")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)

	ctx.Req.Form.Set("collaborator", "user34")

	repo := &context.Repository{
		Owner: &models.User{
			LowerName: "user2",
		},
	}

	ctx.Repo = repo

	CollaborationPost(ctx)

	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "org26/repo43")

	ctx.Req.Form.Set("team", "team11")

	org := &models.User{
		LowerName: "org26",
		Type:      models.UserTypeOrganization,
	}

	team := &models.Team{
		ID:    11,
		OrgID: 26,
	}

	re := &models.Repository{
		ID:      43,
		Owner:   org,
		OwnerID: 26,
	}

	repo := &context.Repository{
		Owner: &models.User{
			ID:                        26,
			LowerName:                 "org26",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)

	assert.True(t, team.HasRepository(re.ID))
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.Empty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost_NotAllowed(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "org26/repo43")

	ctx.Req.Form.Set("team", "team11")

	org := &models.User{
		LowerName: "org26",
		Type:      models.UserTypeOrganization,
	}

	team := &models.Team{
		ID:    11,
		OrgID: 26,
	}

	re := &models.Repository{
		ID:      43,
		Owner:   org,
		OwnerID: 26,
	}

	repo := &context.Repository{
		Owner: &models.User{
			ID:                        26,
			LowerName:                 "org26",
			RepoAdminChangeTeamAccess: false,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)

	assert.False(t, team.HasRepository(re.ID))
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)

}

func TestAddTeamPost_AddTeamTwice(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "org26/repo43")

	ctx.Req.Form.Set("team", "team11")

	org := &models.User{
		LowerName: "org26",
		Type:      models.UserTypeOrganization,
	}

	team := &models.Team{
		ID:    11,
		OrgID: 26,
	}

	re := &models.Repository{
		ID:      43,
		Owner:   org,
		OwnerID: 26,
	}

	repo := &context.Repository{
		Owner: &models.User{
			ID:                        26,
			LowerName:                 "org26",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)

	AddTeamPost(ctx)
	assert.True(t, team.HasRepository(re.ID))
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestAddTeamPost_NonExistentTeam(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "org26/repo43")

	ctx.Req.Form.Set("team", "team-non-existent")

	org := &models.User{
		LowerName: "org26",
		Type:      models.UserTypeOrganization,
	}

	re := &models.Repository{
		ID:      43,
		Owner:   org,
		OwnerID: 26,
	}

	repo := &context.Repository{
		Owner: &models.User{
			ID:                        26,
			LowerName:                 "org26",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	AddTeamPost(ctx)
	assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestDeleteTeam(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "org3/team1/repo3")

	ctx.Req.Form.Set("id", "2")

	org := &models.User{
		LowerName: "org3",
		Type:      models.UserTypeOrganization,
	}

	team := &models.Team{
		ID:    2,
		OrgID: 3,
	}

	re := &models.Repository{
		ID:      3,
		Owner:   org,
		OwnerID: 3,
	}

	repo := &context.Repository{
		Owner: &models.User{
			ID:                        3,
			LowerName:                 "org3",
			RepoAdminChangeTeamAccess: true,
		},
		Repository: re,
	}

	ctx.Repo = repo

	DeleteTeam(ctx)

	assert.False(t, team.HasRepository(re.ID))
}
