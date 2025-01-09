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
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/require"
)

func TestRepoSyncFork(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		forkUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

		baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		baseUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: baseRepo.OwnerID})
		repoString := baseUser.Name + "/" + baseRepo.Name

		session := loginUser(t, forkUser.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		checkSyncForkMessage := func(hasMessage bool, message string) {
			require.Eventually(t, func() bool {
				resp := session.MakeRequest(t, NewRequestf(t, "GET", "/%s/test-repo-fork", forkUser.Name), http.StatusOK)
				htmlDoc := NewHTMLParser(t, resp.Body)
				respMsg, _ := htmlDoc.Find(".ui.message").Html()
				return strings.Contains(respMsg, message) != hasMessage
			}, 5*time.Second, 100*time.Millisecond)
		}

		// create a fork
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/forks", repoString), &api.CreateForkOption{
			Name: util.ToPointer("test-repo-fork"),
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusAccepted)
		forkRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: forkUser.ID, Name: "test-repo-fork"})

		// Case 0: Update fork, should have no sync fork message
		require.NoError(t, createOrReplaceFileInBranch(baseUser, baseRepo, "file0.txt", "master", "dummy"))
		// the repo shows a prompt to "sync fork" and with precise commit count
		checkSyncForkMessage(false, fmt.Sprintf(`<a href="/%v/src/branch/master">%v:master</a>`, repoString, repoString))

		// Case 2: Base is ahead of fork
		require.NoError(t, createOrReplaceFileInBranch(baseUser, baseRepo, "file1.txt", "master", "dummy"))
		// the repo shows a prompt to "sync fork" and with precise commit count
		checkSyncForkMessage(true, fmt.Sprintf(`This branch is 1 commit behind <a href="/%v/src/branch/master">%v:master</a>`, repoString, repoString))

		// Case 3: Base has some commits that fork does not have, but fork updated
		require.NoError(t, createOrReplaceFileInBranch(forkUser, forkRepo, "file2.txt", "master", "dummy"))
		// the repo shows a prompt to "sync fork" and with just "new changes" text
		checkSyncForkMessage(true, fmt.Sprintf(`The base branch <a href="/%v/src/branch/master">%v:master</a> has new changes`, repoString, repoString))

		// Case 4: Base updates again
		require.NoError(t, createOrReplaceFileInBranch(forkUser, forkRepo, "file3.txt", "master", "dummy"))
		// the repo shows a prompt to "sync fork" and with just "new changes" text
		checkSyncForkMessage(true, fmt.Sprintf(`The base branch <a href="/%v/src/branch/master">%v:master</a> has new changes`, repoString, repoString))
	})
}
