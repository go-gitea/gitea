// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
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
	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, migrations.Init())

	_ = db.TruncateBeans(db.DefaultContext, &repo_model.PushMirror{})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	srcRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	mirrorRepo, err := repo_service.CreateRepositoryDirectly(db.DefaultContext, user, user, repo_service.CreateRepoOptions{
		Name: "test-push-mirror",
	})
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	pushMirrorURL := fmt.Sprintf("%s%s/%s", u.String(), url.PathEscape(user.Name), url.PathEscape(mirrorRepo.Name))
	testCreatePushMirror(t, session, user.Name, srcRepo.Name, pushMirrorURL, user.LowerName, userPassword, "0")

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
	assert.True(t, doRemovePushMirror(t, session, user.Name, srcRepo.Name, mirrors[0].ID))
	mirrors, _, err = repo_model.GetPushMirrorsByRepoID(db.DefaultContext, srcRepo.ID, db.ListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, mirrors)
}

func testCreatePushMirror(t *testing.T, session *TestSession, owner, repo, address, username, password, interval string) {
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings", url.PathEscape(owner), url.PathEscape(repo)), map[string]string{
		"_csrf":                GetUserCSRFToken(t, session),
		"action":               "push-mirror-add",
		"push_mirror_address":  address,
		"push_mirror_username": username,
		"push_mirror_password": password,
		"push_mirror_interval": interval,
	})
	session.MakeRequest(t, req, http.StatusSeeOther)

	flashCookie := session.GetCookie(gitea_context.CookieNameFlash)
	assert.NotNil(t, flashCookie)
	assert.Contains(t, flashCookie.Value, "success")
}

func doRemovePushMirror(t *testing.T, session *TestSession, owner, repo string, pushMirrorID int64) bool {
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings", url.PathEscape(owner), url.PathEscape(repo)), map[string]string{
		"_csrf":          GetUserCSRFToken(t, session),
		"action":         "push-mirror-remove",
		"push_mirror_id": strconv.FormatInt(pushMirrorID, 10),
	})
	resp := session.MakeRequest(t, req, NoExpectedStatus)
	flashCookie := session.GetCookie(gitea_context.CookieNameFlash)
	return resp.Code == http.StatusSeeOther && flashCookie != nil && strings.Contains(flashCookie.Value, "success")
}

func doUpdatePushMirror(t *testing.T, session *TestSession, owner, repo string, pushMirrorID int64, interval string) bool {
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings", owner, repo), map[string]string{
		"_csrf":                  GetUserCSRFToken(t, session),
		"action":                 "push-mirror-update",
		"push_mirror_id":         strconv.FormatInt(pushMirrorID, 10),
		"push_mirror_interval":   interval,
		"push_mirror_defer_sync": "true",
	})
	resp := session.MakeRequest(t, req, NoExpectedStatus)
	return resp.Code == http.StatusSeeOther
}

func TestRepoSettingPushMirrorUpdate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, migrations.Init())

	session := loginUser(t, "user2")
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	testCreatePushMirror(t, session, "user2", "repo2", "https://127.0.0.1/user1/repo1.git", "", "", "24h")

	pushMirrors, cnt, err := repo_model.GetPushMirrorsByRepoID(db.DefaultContext, repo2.ID, db.ListOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
	assert.EqualValues(t, 24*time.Hour, pushMirrors[0].Interval)
	repo2PushMirrorID := pushMirrors[0].ID

	// update repo2 push mirror
	assert.True(t, doUpdatePushMirror(t, session, "user2", "repo2", repo2PushMirrorID, "10m0s"))
	pushMirror := unittest.AssertExistsAndLoadBean(t, &repo_model.PushMirror{ID: repo2PushMirrorID})
	assert.EqualValues(t, 10*time.Minute, pushMirror.Interval)

	// avoid updating repo2 push mirror from repo1
	assert.False(t, doUpdatePushMirror(t, session, "user2", "repo1", repo2PushMirrorID, "20m0s"))
	pushMirror = unittest.AssertExistsAndLoadBean(t, &repo_model.PushMirror{ID: repo2PushMirrorID})
	assert.EqualValues(t, 10*time.Minute, pushMirror.Interval) // not changed

	// avoid deleting repo2 push mirror from repo1
	assert.False(t, doRemovePushMirror(t, session, "user2", "repo1", repo2PushMirrorID))
	unittest.AssertExistsAndLoadBean(t, &repo_model.PushMirror{ID: repo2PushMirrorID})

	// delete repo2 push mirror
	assert.True(t, doRemovePushMirror(t, session, "user2", "repo2", repo2PushMirrorID))
	unittest.AssertNotExistsBean(t, &repo_model.PushMirror{ID: repo2PushMirrorID})
}
