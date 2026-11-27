// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"testing"

	secret_model "gitea.dev/models/secret"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestActionsSecretsSettings(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	for _, scope := range []struct {
		name    string
		link    string
		ownerID int64
		repoID  int64
	}{
		{"User", "/user/settings/actions/secrets", 2, 0},
		{"Org", "/org/org3/settings/actions/secrets", 3, 0},
		{"Repo", "/user2/repo1/settings/actions/secrets", 0, 1},
	} {
		t.Run(scope.name, func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", scope.link, map[string]string{"name": "SCOPED_SECRET", "data": "data"})
			resp := session.MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, scope.link, test.RedirectURL(resp))
			secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{Name: "SCOPED_SECRET"})
			assert.Equal(t, scope.ownerID, secret.OwnerID)
			assert.Equal(t, scope.repoID, secret.RepoID)

			resp = session.MakeRequest(t, NewRequest(t, "GET", scope.link), http.StatusOK)
			assert.Contains(t, resp.Body.String(), "SCOPED_SECRET")

			req = NewRequestWithValues(t, "POST", scope.link+"/delete", map[string]string{"id": strconv.FormatInt(secret.ID, 10)})
			session.MakeRequest(t, req, http.StatusOK)
			unittest.AssertNotExistsBean(t, &secret_model.Secret{ID: secret.ID})
		})
	}
}
