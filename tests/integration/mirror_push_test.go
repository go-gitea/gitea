// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	gitea_context "code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/migrations"
	mirror_service "code.gitea.io/gitea/services/mirror"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestMirrorPush(t *testing.T) {
	onGiteaRun(t, testMirrorPush)
}

func testMirrorPush(t *testing.T, u *url.URL) {
	defer tests.PrepareTestEnv(t)()

	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, migrations.Init())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	srcRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	mirrorRepo, err := repo_service.CreateRepositoryDirectly(db.DefaultContext, user, user, repo_service.CreateRepoOptions{
		Name: "test-push-mirror",
	})
	assert.NoError(t, err)

	ctx := NewAPITestContext(t, user.LowerName, srcRepo.Name)

	doCreatePushMirror(ctx, fmt.Sprintf("%s%s/%s", u.String(), url.PathEscape(ctx.Username), url.PathEscape(mirrorRepo.Name)), user.LowerName, userPassword)(t)

	mirrors, _, err := repo_model.GetPushMirrorsByRepoID(db.DefaultContext, srcRepo.ID, db.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, mirrors, 1)

	ok := mirror_service.SyncPushMirror(context.Background(), mirrors[0].ID)
	assert.True(t, ok)

	srcGitRepo, err := gitrepo.OpenRepository(git.DefaultContext, srcRepo)
	assert.NoError(t, err)
	defer srcGitRepo.Close()

	srcCommit, err := srcGitRepo.GetBranchCommit("master")
	assert.NoError(t, err)

	mirrorGitRepo, err := gitrepo.OpenRepository(git.DefaultContext, mirrorRepo)
	assert.NoError(t, err)
	defer mirrorGitRepo.Close()

	mirrorCommit, err := mirrorGitRepo.GetBranchCommit("master")
	assert.NoError(t, err)

	assert.Equal(t, srcCommit.ID, mirrorCommit.ID)

	// Cleanup
	doRemovePushMirror(ctx, fmt.Sprintf("%s%s/%s", u.String(), url.PathEscape(ctx.Username), url.PathEscape(mirrorRepo.Name)), user.LowerName, userPassword, int(mirrors[0].ID))(t)
	mirrors, _, err = repo_model.GetPushMirrorsByRepoID(db.DefaultContext, srcRepo.ID, db.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, mirrors, 0)
}

func doCreatePushMirror(ctx APITestContext, address, username, password string) func(t *testing.T) {
	return func(t *testing.T) {
		csrf := GetUserCSRFToken(t, ctx.Session)

		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)), map[string]string{
			"_csrf":                csrf,
			"action":               "push-mirror-add",
			"push_mirror_address":  address,
			"push_mirror_username": username,
			"push_mirror_password": password,
			"push_mirror_interval": "0",
		})
		ctx.Session.MakeRequest(t, req, http.StatusSeeOther)

		flashCookie := ctx.Session.GetCookie(gitea_context.CookieNameFlash)
		assert.NotNil(t, flashCookie)
		assert.Contains(t, flashCookie.Value, "success")
	}
}

func doRemovePushMirror(ctx APITestContext, address, username, password string, pushMirrorID int) func(t *testing.T) {
	return func(t *testing.T) {
		csrf := GetUserCSRFToken(t, ctx.Session)

		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)), map[string]string{
			"_csrf":                csrf,
			"action":               "push-mirror-remove",
			"push_mirror_id":       strconv.Itoa(pushMirrorID),
			"push_mirror_address":  address,
			"push_mirror_username": username,
			"push_mirror_password": password,
			"push_mirror_interval": "0",
		})
		ctx.Session.MakeRequest(t, req, http.StatusSeeOther)

		flashCookie := ctx.Session.GetCookie(gitea_context.CookieNameFlash)
		assert.NotNil(t, flashCookie)
		assert.Contains(t, flashCookie.Value, "success")
	}
}

func TestRepoSettingPushMirror(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	repoPrefix := "/user2/repo2"
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})

	defer test.MockVariableValue(&setting.Migrations.AllowedDomains, "127.0.0.1")()
	assert.NoError(t, migrations.Init())
	defer func() {
		migrations.Init()
	}()

	// visit repository setting page
	req := NewRequest(t, "GET", repoPrefix+"/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	defer func() {
		// avoid dirty mirror data once test failure
		repo_model.DeletePushMirrors(db.DefaultContext, repo_model.PushMirrorOptions{
			RepoID: repo2.ID,
		})
	}()

	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		t.Run("Push Mirror Add", func(t *testing.T) {
			req = NewRequestWithValues(t, "POST", repoPrefix+"/settings", map[string]string{
				"_csrf":                htmlDoc.GetCSRF(),
				"action":               "push-mirror-add",
				"push_mirror_address":  u.String() + "/user1/repo1.git",
				"push_mirror_interval": "0",
			})
			session.MakeRequest(t, req, http.StatusSeeOther)

			flashCookie := session.GetCookie(gitea_context.CookieNameFlash)
			assert.NotNil(t, flashCookie)
			assert.Contains(t, flashCookie.Value, "success")

			mirrors, cnt, err := repo_model.GetPushMirrorsByRepoID(db.DefaultContext, repo2.ID, db.ListOptions{})
			assert.NoError(t, err)
			assert.Len(t, mirrors, 1)
			assert.EqualValues(t, 1, cnt)
			assert.EqualValues(t, 0, mirrors[0].Interval)
		})

		mirrors, _, _ := repo_model.GetPushMirrorsByRepoID(db.DefaultContext, repo2.ID, db.ListOptions{})

		t.Run("Push Mirror Update", func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", repoPrefix+"/settings", map[string]string{
				"_csrf":                htmlDoc.GetCSRF(),
				"action":               "push-mirror-update",
				"push_mirror_id":       strconv.FormatInt(mirrors[0].ID, 10),
				"push_mirror_interval": "10m0s",
			})
			session.MakeRequest(t, req, http.StatusSeeOther)

			mirror, err := repo_model.GetPushMirrorByID(db.DefaultContext, mirrors[0].ID)
			assert.NoError(t, err)
			assert.EqualValues(t, 10*time.Minute, mirror.Interval)

			req = NewRequestWithValues(t, "POST", repoPrefix+"/settings", map[string]string{
				"_csrf":                htmlDoc.GetCSRF(),
				"action":               "push-mirror-update",
				"push_mirror_id":       strconv.FormatInt(9999, 10), // 1 is an mirror ID which is not exist
				"push_mirror_interval": "10m0s",
			})
			session.MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run("Push Mirror Remove", func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", repoPrefix+"/settings", map[string]string{
				"_csrf":          htmlDoc.GetCSRF(),
				"action":         "push-mirror-remove",
				"push_mirror_id": strconv.FormatInt(mirrors[0].ID, 10),
			})
			session.MakeRequest(t, req, http.StatusSeeOther)

			_, err := repo_model.GetPushMirrorByID(db.DefaultContext, mirrors[0].ID)
			assert.Error(t, err)
		})
	})
}
