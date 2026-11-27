// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
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
	const secretName = "SCOPED_SECRET"

	for _, scope := range []struct {
		name, link      string
		ownerID, repoID int64
	}{
		{"User", "/user/settings/actions/secrets", 2, 0},
		{"Org", "/org/org3/settings/actions/secrets", 3, 0},
		{"Repo", "/user2/repo1/settings/actions/secrets", 0, 1},
	} {
		t.Run(scope.name, func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", scope.link, map[string]string{"name": secretName, "data": "data"})
			resp := session.MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, scope.link, test.RedirectURL(resp))
			secret := unittest.AssertExistsAndLoadBean(t, &secret_model.Secret{Name: secretName})
			assert.Equal(t, scope.ownerID, secret.OwnerID)
			assert.Equal(t, scope.repoID, secret.RepoID)

			deleteURL := fmt.Sprintf("%s/delete?id=%d", scope.link, secret.ID)
			deleteButton := `button[data-url="` + deleteURL + `"]`
			resp = session.MakeRequest(t, NewRequest(t, "GET", scope.link), http.StatusOK)
			AssertHTMLElement(t, NewHTMLParser(t, resp.Body), deleteButton, 1)

			session.MakeRequest(t, NewRequest(t, "POST", deleteURL), http.StatusOK)
			unittest.AssertNotExistsBean(t, &secret_model.Secret{ID: secret.ID})
			resp = session.MakeRequest(t, NewRequest(t, "GET", scope.link), http.StatusOK)
			AssertHTMLElement(t, NewHTMLParser(t, resp.Body), deleteButton, 0)
		})
	}
}
