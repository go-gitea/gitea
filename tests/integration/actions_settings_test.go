// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestActionsCollaborativeOwner(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// actionRepo is a private repo and its owner is org3
		actionRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})

		// user2 is an admin of org3
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		// a private repo(id=6) of user10 will try to clone actionRepo
		user10 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 10})

		taskToken := "674f727a81ed2f195bccab036cccf86a182199eb" // task id is 49
		u.Path = fmt.Sprintf("%s/%s.git", actionRepo.OwnerName, actionRepo.Name)
		u.User = url.UserPassword(taskToken, "")

		// now user10 is not a collaborative owner, so the git clone will fail
		doGitCloneFail(u)(t)

		// add user10 to the list of collaborative owners
		user2Session := loginUser(t, user2.Name)
		user2CSRF := GetUserCSRFToken(t, user2Session)
		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/add", actionRepo.OwnerName, actionRepo.Name), map[string]string{
			"_csrf":               user2CSRF,
			"collaborative_owner": user10.Name,
		})
		user2Session.MakeRequest(t, req, http.StatusSeeOther)

		// the git clone will be successful
		doGitClone(t.TempDir(), u)(t)

		// remove user10 from the list of collaborative owners
		req = NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/actions/general/collaborative_owner/delete", actionRepo.OwnerName, actionRepo.Name), map[string]string{
			"_csrf": user2CSRF,
			"id":    fmt.Sprintf("%d", user10.ID),
		})
		resp := user2Session.MakeRequest(t, req, http.StatusOK)
		res := make(map[string]string)
		assert.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.EqualValues(t, fmt.Sprintf("/%s/%s/settings/actions/general", actionRepo.OwnerName, actionRepo.Name), res["redirect"])

		// the git clone will fail
		doGitCloneFail(u)(t)
	})
}
