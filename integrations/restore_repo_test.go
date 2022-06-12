// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/private"

	"github.com/stretchr/testify/assert"
	"lab.forgefriends.org/friendlyforgeformat/gofff"
	"lab.forgefriends.org/friendlyforgeformat/gofff/forges/file"
)

func TestAPIPrivateRestoreRepo(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		fixture := file.NewFixture(t, gofff.AllFeatures)
		fixture.CreateEverything(file.User1)

		repoOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)

		repoName := "restoredrepo"
		validation := true
		statusCode, errStr := private.RestoreRepo(
			context.Background(),
			fixture.GetDirectory(),
			repoOwner.Name,
			repoName,
			[]string{"issues"},
			validation,
		)
		assert.EqualValues(t, http.StatusOK, statusCode, errStr)

		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: repoName})
	})
}
