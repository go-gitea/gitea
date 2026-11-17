// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git/gitrepo"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestLockFiles creates an old (stale) and a fresh lock file in the git repository of repo1, and returns their paths
func createTestLockFiles(t *testing.T) (stale, fresh string) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	refsDir := filepath.Join(gitrepo.RepoLocalPath(repo), "refs", "heads")
	stale, fresh = filepath.Join(refsDir, "stale-branch.lock"), filepath.Join(refsDir, "fresh-branch.lock")
	for _, p := range []string{stale, fresh} {
		require.NoError(t, os.WriteFile(p, nil, 0o644))
	}
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(stale, old, old))
	return stale, fresh
}

func TestAPIRepoMaintenance(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ownerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	siteAdminToken := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteRepository)

	t.Run("SyncBranches", func(t *testing.T) {
		// a repo owner can sync
		_, err := db.GetEngine(t.Context()).Where("repo_id = ? AND name = ?", 1, "branch2").Delete(&git_model.Branch{})
		require.NoError(t, err)
		unittest.AssertNotExistsBean(t, &git_model.Branch{RepoID: 1, Name: "branch2"})

		req := NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/sync-branches").AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusNoContent)
		unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: 1, Name: "branch2"})

		// and so can a site admin
		_, err = db.GetEngine(t.Context()).Where("repo_id = ? AND name = ?", 1, "branch2").Delete(&git_model.Branch{})
		require.NoError(t, err)
		req = NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/sync-branches").AddTokenAuth(siteAdminToken)
		MakeRequest(t, req, http.StatusNoContent)
		unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: 1, Name: "branch2"})
	})

	t.Run("SyncTags", func(t *testing.T) {
		unittest.AssertExistsAndLoadBean(t, &repo_model.Release{RepoID: 1, TagName: "delete-tag"})

		req := NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/sync-tags").AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusNoContent)
		unittest.AssertNotExistsBean(t, &repo_model.Release{RepoID: 1, TagName: "delete-tag"})
		unittest.AssertExistsAndLoadBean(t, &repo_model.Release{RepoID: 1, TagName: "v1.1"}) // a real release without a git tag is kept
	})

	t.Run("RemoveStaleLocks", func(t *testing.T) {
		stale, fresh := createTestLockFiles(t)
		req := NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/remove-stale-locks").AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusNoContent)
		assert.NoFileExists(t, stale)
		assert.FileExists(t, fresh) // might be held by a running git process
		require.NoError(t, os.Remove(fresh))
	})

	t.Run("GarbageCollect", func(t *testing.T) {
		req := NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/gc").AddTokenAuth(ownerToken)
		MakeRequest(t, req, http.StatusNoContent)
	})

	t.Run("EmptyRepo", func(t *testing.T) {
		userToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{Name: "maintenance-empty"}).AddTokenAuth(userToken)
		MakeRequest(t, req, http.StatusCreated)

		for _, action := range []string{"sync-branches", "sync-tags", "remove-stale-locks", "gc"} {
			req = NewRequest(t, "POST", "/api/v1/repos/user2/maintenance-empty/maintenance/"+action).AddTokenAuth(ownerToken)
			MakeRequest(t, req, http.StatusNoContent)
		}
	})

	for _, action := range []string{"sync-branches", "sync-tags", "remove-stale-locks", "gc"} {
		t.Run("Errors/"+action, func(t *testing.T) {
			req := NewRequest(t, "POST", "/api/v1/repos/user2/no-such-repo/maintenance/"+action).AddTokenAuth(ownerToken)
			MakeRequest(t, req, http.StatusNotFound)

			// not authenticated
			req = NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/"+action)
			MakeRequest(t, req, http.StatusUnauthorized)

			// authenticated but neither repo admin nor site admin
			otherToken := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteRepository)
			req = NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/"+action).AddTokenAuth(otherToken)
			MakeRequest(t, req, http.StatusForbidden)

			// token without repository write scope
			readToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
			req = NewRequest(t, "POST", "/api/v1/repos/user2/repo1/maintenance/"+action).AddTokenAuth(readToken)
			MakeRequest(t, req, http.StatusForbidden)
		})
	}
}

func TestRepoSettingsMaintenance(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	t.Run("SyncBranches", func(t *testing.T) {
		_, err := db.GetEngine(t.Context()).Where("repo_id = ? AND name = ?", 1, "branch2").Delete(&git_model.Branch{})
		require.NoError(t, err)

		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings", map[string]string{"action": "maintenance-sync-branches"})
		session.MakeRequest(t, req, http.StatusSeeOther)
		unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: 1, Name: "branch2"})
	})

	t.Run("SyncTags", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings", map[string]string{"action": "maintenance-sync-tags"})
		session.MakeRequest(t, req, http.StatusSeeOther)
		unittest.AssertNotExistsBean(t, &repo_model.Release{RepoID: 1, TagName: "delete-tag"})
	})

	t.Run("RemoveStaleLocks", func(t *testing.T) {
		stale, fresh := createTestLockFiles(t)
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings", map[string]string{"action": "maintenance-remove-stale-locks"})
		session.MakeRequest(t, req, http.StatusSeeOther)
		assert.NoFileExists(t, stale)
		assert.FileExists(t, fresh)
		require.NoError(t, os.Remove(fresh))
	})

	t.Run("GarbageCollect", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings", map[string]string{"action": "maintenance-gc"})
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/user2/repo1/settings", resp.Header().Get("Location"))
		flash := session.GetCookieFlashMessage()
		assert.NotEmpty(t, flash.SuccessMsg)
		assert.Empty(t, flash.ErrorMsg)
	})

	t.Run("PageShowsMaintenance", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/settings")
		resp := session.MakeRequest(t, req, http.StatusOK)
		for _, action := range []string{"maintenance-sync-branches", "maintenance-sync-tags", "maintenance-remove-stale-locks", "maintenance-gc"} {
			assert.Contains(t, resp.Body.String(), `value="`+action+`"`)
		}
	})

	t.Run("NonAdminForbidden", func(t *testing.T) {
		other := loginUser(t, "user4")
		req := NewRequestWithValues(t, "POST", "/user2/repo1/settings", map[string]string{"action": "maintenance-gc"})
		other.MakeRequest(t, req, http.StatusNotFound)
	})
}
