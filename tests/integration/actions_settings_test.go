// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestActionsCollaborativeOwner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// user2 is the owner of "reusable_workflow" repo
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user2Session := loginUser(t, user2.Name)
		user2Token := getTokenForLoggedInUser(t, user2Session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
		repo := createActionsTestRepo(t, user2Token, "reusable_workflow", true)

		// a private repo(id=6) of user10 will try to clone "reusable_workflow" repo
		user10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})
		// task id is 55 and its repo_id=6
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 55, RepoID: 6})
		taskToken := "674f727a81ed2f195bccab036cccf86a182199eb"
		tokenHash := auth_model.HashToken(taskToken, task.TokenSalt)
		assert.Equal(t, task.TokenHash, tokenHash)

		dstPath := t.TempDir()
		u.Path = fmt.Sprintf("%s/%s.git", repo.Owner.UserName, repo.Name)
		u.User = url.UserPassword("gitea-actions", taskToken)

		// the git clone will fail
		doGitCloneFail(u)(t)

		// add user10 to the list of collaborative owners
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/add", repo.Owner.UserName, repo.Name), map[string]string{
			"collaborative_owner": user10.Name,
		})
		user2Session.MakeRequest(t, req, http.StatusOK)

		// the git clone will be successful
		doGitClone(dstPath, u)(t)

		// remove user10 from the list of collaborative owners
		req = NewRequest(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/delete?id=%d", repo.Owner.UserName, repo.Name, user10.ID))
		user2Session.MakeRequest(t, req, http.StatusOK)

		// the git clone will fail
		doGitCloneFail(u)(t)
	})
}
