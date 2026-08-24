// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
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

		MakeRequest(t, NewRequestWithJSON(t, "PATCH", base, &api.EditRepoOption{
			ImmutableReleases: new(true),
		}).AddTokenAuth(token), http.StatusOK)
		rel := publish("imm-1")
		assert.True(t, rel.IsImmutable)

		t.Run("API", func(t *testing.T) {
			// a repo edit that leaves the setting out must not clear it
			var apiRepo api.Repository
			DecodeJSON(t, MakeRequest(t, NewRequestWithJSON(t, "PATCH", base, &api.EditRepoOption{
				HasReleases: new(true),
			}).AddTokenAuth(token), http.StatusOK), &apiRepo)
			assert.True(t, apiRepo.ImmutableReleases)

			relURL := fmt.Sprintf("%s/releases/%d", base, rel.ID)

			// the guard rejects the upload before the body is read
			MakeRequest(t, NewRequest(t, "POST", relURL+"/assets?name=a.txt").AddTokenAuth(token), http.StatusUnprocessableEntity)

			// the tag is locked, the title stays editable
			MakeRequest(t, NewRequestWithJSON(t, "PATCH", relURL, &api.EditReleaseOption{
				TagName: "imm-2",
			}).AddTokenAuth(token), http.StatusUnprocessableEntity)
			MakeRequest(t, NewRequestWithJSON(t, "PATCH", relURL, &api.EditReleaseOption{
				Title: "renamed",
			}).AddTokenAuth(token), http.StatusOK)
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

			// once the release is gone the tag itself may be deleted
			MakeRequest(t, NewRequest(t, "DELETE", fmt.Sprintf("%s/releases/%d", base, pushed.ID)).AddTokenAuth(token), http.StatusNoContent)
			_, _, err = gitcmd.NewCommand("push", "origin", ":refs/tags/imm-push").WithDir(dstPath).RunStdString(t.Context())
			assert.NoError(t, err)

			// pushing the tag of a draft publishes it, which must lock it like any other publication
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
