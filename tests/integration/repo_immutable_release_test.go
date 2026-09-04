// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git/gitcmd"
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

		publish := func(tagName string) *api.Release {
			return createNewReleaseUsingAPI(t, token, owner, repo, tagName, "master", tagName, "")
		}
		patchRepo := func(opts *api.EditRepoOption) *httptest.ResponseRecorder {
			return MakeRequest(t, NewRequestWithJSON(t, "PATCH", base, opts).AddTokenAuth(token), http.StatusOK)
		}

		patchRepo(&api.EditRepoOption{ImmutableReleases: new(true)})
		rel := publish("imm-1")
		assert.True(t, rel.IsImmutable)

		t.Run("EditPage", func(t *testing.T) {
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/releases/edit/imm-1", owner.Name, repo.Name))
			htmlDoc := NewHTMLParser(t, session.MakeRequest(t, req, http.StatusOK).Body)
			assert.Contains(t, htmlDoc.doc.Find(".ui.info.message").Text(), "You cannot change the tag, target or assets")
			assert.Equal(t, 0, htmlDoc.doc.Find(".dropzone").Length())
		})

		t.Run("API", func(t *testing.T) {
			var apiRepo api.Repository
			DecodeJSON(t, patchRepo(&api.EditRepoOption{HasReleases: new(true)}), &apiRepo)
			assert.True(t, apiRepo.ImmutableReleases)

			relURL := fmt.Sprintf("%s/releases/%d", base, rel.ID)

			MakeRequest(t, NewRequest(t, "POST", relURL+"/assets?name=a.txt").AddTokenAuth(token), http.StatusUnprocessableEntity)

			MakeRequest(t, NewRequestWithJSON(t, "PATCH", relURL, &api.EditReleaseOption{
				TagName: "imm-2",
			}).AddTokenAuth(token), http.StatusUnprocessableEntity)

			patchRepo(&api.EditRepoOption{HasReleases: new(false)})
			DecodeJSON(t, patchRepo(&api.EditRepoOption{ImmutableReleases: new(true)}), &apiRepo)
			assert.False(t, apiRepo.HasReleases)

			patchRepo(&api.EditRepoOption{HasReleases: new(true), ImmutableReleases: new(true)})

			releases := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypeReleases})
			releases.EveryoneAccessMode = perm.AccessModeRead
			_, err := db.GetEngine(t.Context()).ID(releases.ID).Cols("everyone_access_mode").Update(releases)
			require.NoError(t, err)
			patchRepo(&api.EditRepoOption{ImmutableReleases: new(true)})
			after := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repo.ID, Type: unit.TypeReleases})
			assert.Equal(t, perm.AccessModeRead, after.EveryoneAccessMode)
		})

		t.Run("GitPush", func(t *testing.T) {
			pushed := publish("imm-push")

			dstPath := t.TempDir()
			u.Path = NewAPITestContext(t, owner.Name, repo.Name).GitPath()
			u.User = url.UserPassword(owner.Name, userPassword)
			doGitClone(dstPath, u)(t)
			doGitCheckoutWriteFileCommit(localGitAddCommitOptions{
				LocalRepoPath:   dstPath,
				CheckoutBranch:  "master",
				TreeFilePath:    "immutable.txt",
				TreeFileContent: "content",
			})(t)

			_, _, err := gitcmd.NewCommand("tag", "imm-push", "--force").WithDir(dstPath).RunStdString(t.Context())
			require.NoError(t, err)
			_, _, err = gitcmd.NewCommand("push", "--force", "origin", "refs/tags/imm-push").WithDir(dstPath).RunStdString(t.Context())
			assert.ErrorContains(t, err, "Tag imm-push is immutable")

			_, _, err = gitcmd.NewCommand("push", "origin", ":refs/tags/imm-push").WithDir(dstPath).RunStdString(t.Context())
			assert.ErrorContains(t, err, "Tag imm-push is immutable")

			MakeRequest(t, NewRequest(t, "DELETE", fmt.Sprintf("%s/releases/%d", base, pushed.ID)).AddTokenAuth(token), http.StatusNoContent)
			_, _, err = gitcmd.NewCommand("push", "origin", ":refs/tags/imm-push").WithDir(dstPath).RunStdString(t.Context())
			assert.NoError(t, err)

			var draft api.Release
			DecodeJSON(t, MakeRequest(t, NewRequestWithJSON(t, "POST", base+"/releases", &api.CreateReleaseOption{
				TagName: "imm-draft", Target: "master", Title: "draft", IsDraft: true,
			}).AddTokenAuth(token), http.StatusCreated), &draft)
			assert.False(t, draft.IsImmutable)

			_, _, err = gitcmd.NewCommand("tag", "imm-draft").WithDir(dstPath).RunStdString(t.Context())
			require.NoError(t, err)
			_, _, err = gitcmd.NewCommand("push", "origin", "refs/tags/imm-draft").WithDir(dstPath).RunStdString(t.Context())
			require.NoError(t, err)

			var published api.Release
			DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s/releases/%d", base, draft.ID)).AddTokenAuth(token), http.StatusOK), &published)
			assert.False(t, published.IsDraft)
			assert.True(t, published.IsImmutable)
		})
	})
}
