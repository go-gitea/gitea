// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoMergeUpstream(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		forkUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		baseUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: baseRepo.OwnerID})

		checkFileContent := func(branch, exp string) {
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/test-repo-fork/raw/branch/%s/new-file.txt", forkUser.Name, branch))
			resp := MakeRequest(t, req, http.StatusOK)
			require.Equal(t, exp, resp.Body.String())
		}

		session := loginUser(t, forkUser.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		// create a fork
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/forks", baseUser.Name, baseRepo.Name), &api.CreateForkOption{
			Name: util.ToPointer("test-repo-fork"),
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusAccepted)
		forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: forkUser.ID, Name: "test-repo-fork"})

		// create fork-branch
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/test-repo-fork/branches/_new/branch/master", forkUser.Name), map[string]string{
			"_csrf":           GetUserCSRFToken(t, session),
			"new_branch_name": "fork-branch",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		queryMergeUpstreamButtonLink := func(htmlDoc *HTMLDoc) string {
			return htmlDoc.Find(`button[data-url*="merge-upstream"]`).AttrOr("data-url", "")
		}

		t.Run("HeadBeforeBase", func(t *testing.T) {
			// add a file in base repo
			require.NoError(t, createOrReplaceFileInBranch(baseUser, baseRepo, "new-file.txt", "master", "test-content-1"))

			// the repo shows a prompt to "sync fork"
			var mergeUpstreamLink string
			require.Eventually(t, func() bool {
				resp := session.MakeRequest(t, NewRequestf(t, "GET", "/%s/test-repo-fork/src/branch/fork-branch", forkUser.Name), http.StatusOK)
				htmlDoc := NewHTMLParser(t, resp.Body)
				mergeUpstreamLink = queryMergeUpstreamButtonLink(htmlDoc)
				if mergeUpstreamLink == "" {
					return false
				}
				respMsg, _ := htmlDoc.Find(".ui.message:not(.positive)").Html()
				return strings.Contains(respMsg, `This branch is 1 commit behind <a href="/user2/repo1/src/branch/master">user2/repo1:master</a>`)
			}, 5*time.Second, 100*time.Millisecond)

			// click the "sync fork" button
			req = NewRequestWithValues(t, "POST", mergeUpstreamLink, map[string]string{"_csrf": GetUserCSRFToken(t, session)})
			session.MakeRequest(t, req, http.StatusOK)
			checkFileContent("fork-branch", "test-content-1")
		})

		t.Run("BaseChangeAfterHeadChange", func(t *testing.T) {
			// update the files: base first, head later, and check the prompt
			require.NoError(t, createOrReplaceFileInBranch(baseUser, baseRepo, "new-file.txt", "master", "test-content-2"))
			require.NoError(t, createOrReplaceFileInBranch(forkUser, forkRepo, "new-file-other.txt", "fork-branch", "test-content-other"))

			// make sure the base branch's update time is before the fork, to make it test the complete logic
			baseBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: baseRepo.ID, Name: "master"})
			forkBranch := unittest.AssertExistsAndLoadBean(t, &git_model.Branch{RepoID: forkRepo.ID, Name: "fork-branch"})
			_, err := db.GetEngine(db.DefaultContext).ID(forkBranch.ID).Update(&git_model.Branch{UpdatedUnix: baseBranch.UpdatedUnix + 1})
			require.NoError(t, err)

			// the repo shows a prompt to "sync fork"
			require.Eventually(t, func() bool {
				resp := session.MakeRequest(t, NewRequestf(t, "GET", "/%s/test-repo-fork/src/branch/fork-branch", forkUser.Name), http.StatusOK)
				htmlDoc := NewHTMLParser(t, resp.Body)
				respMsg, _ := htmlDoc.Find(".ui.message:not(.positive)").Html()
				return strings.Contains(respMsg, `The base branch <a href="/user2/repo1/src/branch/master">user2/repo1:master</a> has new changes`)
			}, 5*time.Second, 100*time.Millisecond)

			// and do the merge-upstream by API
			req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/test-repo-fork/merge-upstream", forkUser.Name), &api.MergeUpstreamRequest{
				Branch: "fork-branch",
			}).AddTokenAuth(token)
			resp := MakeRequest(t, req, http.StatusOK)
			checkFileContent("fork-branch", "test-content-2")

			var mergeResp api.MergeUpstreamResponse
			DecodeJSON(t, resp, &mergeResp)
			assert.Equal(t, "merge", mergeResp.MergeStyle)

			// after merge, there should be no "sync fork" button anymore
			require.Eventually(t, func() bool {
				resp := session.MakeRequest(t, NewRequestf(t, "GET", "/%s/test-repo-fork/src/branch/fork-branch", forkUser.Name), http.StatusOK)
				htmlDoc := NewHTMLParser(t, resp.Body)
				return queryMergeUpstreamButtonLink(htmlDoc) == ""
			}, 5*time.Second, 100*time.Millisecond)
		})
	})
}
