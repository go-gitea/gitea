// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/tests"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
)

func TestActivityPubPerson(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Federation.Enabled, true)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	t.Run("ExistingPerson", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		userID := 2
		username := "user2"
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/activitypub/user-id/%v", userID))
		resp := MakeRequest(t, req, http.StatusOK)
		body := resp.Body.Bytes()
		assert.Contains(t, string(body), "@context")

		var person ap.Person
		err := person.UnmarshalJSON(body)
		assert.NoError(t, err)

		assert.Equal(t, ap.PersonType, person.Type)
		assert.Equal(t, username, person.PreferredUsername.String())
		keyID := person.GetID().String()
		assert.Regexp(t, fmt.Sprintf("activitypub/user-id/%v$", userID), keyID)
		assert.Regexp(t, fmt.Sprintf("activitypub/user-id/%v/outbox$", userID), person.Outbox.GetID().String())
		assert.Regexp(t, fmt.Sprintf("activitypub/user-id/%v/inbox$", userID), person.Inbox.GetID().String())

		pubKey := person.PublicKey
		assert.NotNil(t, pubKey)
		publicKeyID := keyID + "#main-key"
		assert.Equal(t, pubKey.ID.String(), publicKeyID)

		pubKeyPem := pubKey.PublicKeyPem
		assert.NotNil(t, pubKeyPem)
		assert.Regexp(t, "^-----BEGIN PUBLIC KEY-----", pubKeyPem)
	})
	t.Run("MissingPerson", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/api/v1/activitypub/user-id/999999999")
		resp := MakeRequest(t, req, http.StatusNotFound)
		assert.Contains(t, resp.Body.String(), "user does not exist")
	})
	t.Run("MissingPersonInbox", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		srv := httptest.NewServer(testWebRoutes)
		defer srv.Close()
		defer test.MockVariableValue(&setting.AppURL, srv.URL+"/")()

		username1 := "user1"
		ctx := t.Context()
		user1, err := user_model.GetUserByName(ctx, username1)
		assert.NoError(t, err)
		user1url := fmt.Sprintf("%s/api/v1/activitypub/user-id/1#main-key", srv.URL)
		c, err := activitypub.NewClient(db.DefaultContext, user1, user1url)
		assert.NoError(t, err)
		user2inboxurl := fmt.Sprintf("%s/api/v1/activitypub/user-id/2/inbox", srv.URL)

		// Signed request succeeds
		resp, err := c.Post([]byte{}, user2inboxurl)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Unsigned request fails
		req := NewRequest(t, "POST", user2inboxurl)
		MakeRequest(t, req, http.StatusInternalServerError)
	})
}
