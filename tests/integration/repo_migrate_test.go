// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	mirror_service "gitea.dev/services/mirror"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRepoMigrate(t testing.TB, session *TestSession, cloneAddr, repoName string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", fmt.Sprintf("/repo/migrate?service_type=%d", structs.PlainGitService)) // render plain git migration page
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")

	uid, exists := htmlDoc.doc.Find(`input[name="uid"]`).Attr("value")
	assert.True(t, exists, "The template has changed")

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"clone_addr": cloneAddr,
		"uid":        uid,
		"repo_name":  repoName,
		"service":    fmt.Sprintf("%d", structs.PlainGitService),
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func TestRepoMigrate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	resp := testRepoMigrate(t, session, "https://github.com/go-gitea/test_repo.git", "git")
	assert.Equal(t, "/user2/git", *test.ParseJSONRedirect(resp.Body.Bytes()).Redirect)
}

func TestRepoMigrateMirrorOverSSH(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		keypair, err := user_model.CreateSSHKeypair(t.Context(), user2.ID)
		require.NoError(t, err)
		_, err = asymkey_model.AddPublicKey(t.Context(), user2.ID, "managed", keypair.PublicKey, 0, true)
		require.NoError(t, err)
		session := loginUser(t, user2.Name)
		cloneAddr := createSSHUrl("/user2/repo1.git", u).String()

		req := NewRequestWithValues(t, "POST", "/repo/migrate", map[string]string{
			"clone_addr":       cloneAddr,
			"uid":              "2",
			"repo_name":        "foreign-ssh-key",
			"service":          strconv.Itoa(int(structs.PlainGitService)),
			"ssh_key_owner_id": "1",
		})
		resp := session.MakeRequest(t, req, http.StatusBadRequest)
		assert.Contains(t, test.ParseJSONError(resp.Body.Bytes()).ErrorMessage, "You can only use your own managed SSH key")

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		req = NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate", &structs.MigrateRepoOptions{
			CloneAddr: cloneAddr,
			RepoOwner: user2.Name,
			RepoName:  "ssh-mirror",
			Mirror:    true,
			Service:   "git",
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		mirrorRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user2.ID, Name: "ssh-mirror"})
		assert.Equal(t, user2.ID, unittest.AssertExistsAndLoadBean(t, &repo_model.Mirror{RepoID: mirrorRepo.ID}).SSHKeyOwnerID)
		assert.True(t, mirror_service.SyncPullMirror(t.Context(), mirrorRepo.ID))

		srcRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		target, err := repo_service.CreateRepositoryDirectly(t.Context(), user2, user2, repo_service.CreateRepoOptions{Name: "ssh-push-target"}, true)
		require.NoError(t, err)
		testCreatePushMirror(t, session, user2.Name, srcRepo.Name, createSSHUrl("/user2/ssh-push-target.git", u).String(), "", "", "0")
		pushMirrors, _, err := repo_model.GetPushMirrorsByRepoID(t.Context(), srcRepo.ID, db.ListOptions{})
		require.NoError(t, err)
		require.Len(t, pushMirrors, 1)
		assert.Equal(t, user2.ID, pushMirrors[0].SSHKeyOwnerID)
		require.True(t, mirror_service.SyncPushMirror(t.Context(), pushMirrors[0].ID))

		srcCommitID, err := git.GetBranchCommitID(t.Context(), srcRepo, "master")
		require.NoError(t, err)
		targetCommitID, err := git.GetBranchCommitID(t.Context(), target, "master")
		require.NoError(t, err)
		assert.Equal(t, srcCommitID, targetCommitID)
	})
}
