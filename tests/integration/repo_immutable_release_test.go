// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/queue"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImmutableRelease(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		base := fmt.Sprintf("/api/v1/repos/%s/%s", owner.Name, repo.Name)
		patchRepo := func(opts *api.EditRepoOption) *httptest.ResponseRecorder {
			return MakeRequest(t, NewRequestWithJSON(t, "PATCH", base, opts).AddTokenAuth(token), http.StatusOK)
		}

		patchRepo(&api.EditRepoOption{ImmutableReleases: new(true)})
		rel := createNewReleaseUsingAPI(t, token, owner, repo, "imm-1", "master", "imm-1", "")
		assert.True(t, rel.IsImmutable)

		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/releases/edit/imm-1", owner.Name, repo.Name))
		htmlDoc := NewHTMLParser(t, session.MakeRequest(t, req, http.StatusOK).Body)
		assert.Contains(t, htmlDoc.doc.Find(".ui.info.message").Text(), "You cannot change the tag, target or assets")
		assert.Equal(t, 0, htmlDoc.doc.Find(".dropzone").Length())

		relURL := fmt.Sprintf("%s/releases/%d", base, rel.ID)
		MakeRequest(t, NewRequest(t, "POST", relURL+"/assets?name=a.txt").AddTokenAuth(token), http.StatusUnprocessableEntity)

		dstPath := t.TempDir()
		u.Path = repo.FullName() + ".git"
		u.User = url.UserPassword(owner.Name, userPassword)
		doGitClone(dstPath, u)(t)

		_, _, err := gitcmd.NewCommand("tag", "--force", "imm-1", "origin/DefaultBranch").WithDir(dstPath).RunStdString(t.Context())
		require.NoError(t, err)
		_, _, err = gitcmd.NewCommand("push", "--force", "origin", "refs/tags/imm-1").WithDir(dstPath).RunStdString(t.Context())
		assert.ErrorContains(t, err, "Tag imm-1 is immutable")
		_, _, err = gitcmd.NewCommand("push", "origin", ":refs/tags/imm-1").WithDir(dstPath).RunStdString(t.Context())
		assert.ErrorContains(t, err, "Tag imm-1 is immutable")

		MakeRequest(t, NewRequest(t, "DELETE", relURL).AddTokenAuth(token), http.StatusNoContent)
		_, _, err = gitcmd.NewCommand("push", "origin", ":refs/tags/imm-1").WithDir(dstPath).RunStdString(t.Context())
		assert.NoError(t, err)

		var draft api.Release
		DecodeJSON(t, MakeRequest(t, NewRequestWithJSON(t, "POST", base+"/releases", &api.CreateReleaseOption{
			TagName: "imm-draft", Target: "master", Title: "draft", IsDraft: true,
		}).AddTokenAuth(token), http.StatusCreated), &draft)
		_, _, err = gitcmd.NewCommand("tag", "imm-draft").WithDir(dstPath).RunStdString(t.Context())
		require.NoError(t, err)
		_, _, err = gitcmd.NewCommand("push", "origin", "refs/tags/imm-draft").WithDir(dstPath).RunStdString(t.Context())
		require.NoError(t, err)
		require.NoError(t, queue.GetManager().FlushAll(t.Context(), 10*time.Second))
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: draft.ID}).IsDraft)

		patchRepo(&api.EditRepoOption{HasReleases: new(false)})
		var apiRepo api.Repository
		DecodeJSON(t, patchRepo(&api.EditRepoOption{ImmutableReleases: new(true)}), &apiRepo)
		assert.False(t, apiRepo.HasReleases)
	})
}
