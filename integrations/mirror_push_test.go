// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	mirror_service "code.gitea.io/gitea/services/mirror"

	"github.com/stretchr/testify/assert"
)

func TestMirrorPush(t *testing.T) {
	onGiteaRun(t, testMirrorPush)
}

func testMirrorPush(t *testing.T, u *url.URL) {
	defer prepareTestEnv(t)()

	setting.Migrations.AllowLocalNetworks = true

	user := db.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	srcRepo := db.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	mirrorRepo, err := repository.CreateRepository(user, user, models.CreateRepoOptions{
		Name: "test-push-mirror",
	})
	assert.NoError(t, err)

	ctx := NewAPITestContext(t, user.LowerName, srcRepo.Name)

	doCreatePushMirror(ctx, fmt.Sprintf("%s%s/%s", u.String(), url.PathEscape(ctx.Username), url.PathEscape(mirrorRepo.Name)), user.LowerName, userPassword)(t)

	mirrors, err := models.GetPushMirrorsByRepoID(srcRepo.ID)
	assert.NoError(t, err)
	assert.Len(t, mirrors, 1)

	ok := mirror_service.SyncPushMirror(context.Background(), mirrors[0].ID)
	assert.True(t, ok)

	srcGitRepo, err := git.OpenRepository(srcRepo.RepoPath())
	assert.NoError(t, err)
	defer srcGitRepo.Close()

	srcCommit, err := srcGitRepo.GetBranchCommit("master")
	assert.NoError(t, err)

	mirrorGitRepo, err := git.OpenRepository(mirrorRepo.RepoPath())
	assert.NoError(t, err)
	defer mirrorGitRepo.Close()

	mirrorCommit, err := mirrorGitRepo.GetBranchCommit("master")
	assert.NoError(t, err)

	assert.Equal(t, srcCommit.ID, mirrorCommit.ID)
}

func doCreatePushMirror(ctx APITestContext, address, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		csrf := GetCSRF(t, ctx.Session, fmt.Sprintf("/%s/%s/settings", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)))

		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)), map[string]string{
			"_csrf":                csrf,
			"action":               "push-mirror-add",
			"push_mirror_address":  address,
			"push_mirror_username": username,
			"push_mirror_password": password,
			"push_mirror_interval": "0",
		})
		ctx.Session.MakeRequest(t, req, http.StatusFound)

		flashCookie := ctx.Session.GetCookie("macaron_flash")
		assert.NotNil(t, flashCookie)
		assert.Contains(t, flashCookie.Value, "success")
	}
}
