// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/json"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/activitypub"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-fed/activity/pub"
	"github.com/go-fed/activity/streams"
	"github.com/go-fed/activity/streams/vocab"
	"github.com/stretchr/testify/assert"
)

func TestActivityPubPerson(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Federation.Enabled = true
		setting.Database.LogSQL = true
		defer func() {
			setting.Federation.Enabled = false
			setting.Database.LogSQL = false
		}()

		username := "user2"
		req := NewRequestf(t, "GET", fmt.Sprintf("/api/v1/activitypub/user/%s", username))
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, resp.Body.String(), "@context")
		var m map[string]interface{}
		err := json.Unmarshal(resp.Body.Bytes(), &m)

		var person vocab.ActivityStreamsPerson
		resolver, _ := streams.NewJSONResolver(func(c context.Context, p vocab.ActivityStreamsPerson) error {
			person = p
			return nil
		})
		ctx := context.Background()
		err := resolver.Resolve(ctx, m)
		assert.Equal(t, err, nil)
		assert.Equal(t, "Person", person.GetTypeName())
		assert.Equal(t, username, person.GetActivityStreamsName().Begin().GetXMLSchemaString())
		keyID := person.GetJSONLDId().GetIRI().String()
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s$", username), keyID)
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s/outbox$", username), person.GetActivityStreamsOutbox().GetIRI().String())
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s/inbox$", username), person.GetActivityStreamsInbox().GetIRI().String())

		pkp := person.GetW3IDSecurityV1PublicKey()
		assert.NotNil(t, pkp)
		publicKeyID := keyID + "/#main-key"
		var pkpFound vocab.W3IDSecurityV1PublicKey
		for pkpIter := pkp.Begin(); pkpIter != pkp.End(); pkpIter = pkpIter.Next() {
			if !pkpIter.IsW3IDSecurityV1PublicKey() {
				continue
			}
			pkValue := pkpIter.Get()
			var pkID *url.URL
			pkID, err = pub.GetId(pkValue)
			if err != nil {
				return
			}
			assert.Equal(t, pkID.String(), publicKeyID)
			if pkID.String() != publicKeyID {
				continue
			}
			pkpFound = pkValue
			break
		}
		assert.NotNil(t, pkpFound)

		pkPemProp := pkpFound.GetW3IDSecurityV1PublicKeyPem()
		assert.NotNil(t, pkPemProp)
		assert.True(t, pkPemProp.IsXMLSchemaString())

		pubKeyPem := pkPemProp.Get()
		assert.Regexp(t, "^-----BEGIN PUBLIC KEY-----", pubKeyPem)
	})
}

func TestActivityPubMissingPerson(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Federation.Enabled = true
		defer func() {
			setting.Federation.Enabled = false
		}()

		req := NewRequestf(t, "GET", "/api/v1/activitypub/user/nonexistentuser")
		resp := MakeRequest(t, req, http.StatusNotFound)
		assert.Contains(t, resp.Body.String(), "GetUserByName")
	})
}

func TestActivityPubPersonInbox(t *testing.T) {
	srv := httptest.NewServer(c)
	defer srv.Close()

	onGiteaRun(t, func(*testing.T, *url.URL) {
		appURL := setting.AppURL
		setting.Federation.Enabled = true
		setting.Database.LogSQL = true
		setting.AppURL = srv.URL
		defer func() {
			setting.Federation.Enabled = false
			setting.Database.LogSQL = false
			setting.AppURL = appURL
		}()
		username1 := "user1"
		user1, err := user_model.GetUserByName(username1)
		assert.NoError(t, err)
		user1url := fmt.Sprintf("%s/api/v1/activitypub/user/%s/#main-key", srv.URL, username1)
		c, err := activitypub.NewClient(user1, user1url)
		assert.NoError(t, err)
		username2 := "user2"
		user2inboxurl := fmt.Sprintf("%s/api/v1/activitypub/user/%s/inbox", srv.URL, username2)

		// Signed request succeeds
		resp, err := c.Post([]byte{}, user2inboxurl)
		assert.NoError(t, err)
		assert.Equal(t, 204, resp.StatusCode)

		// Unsigned request fails
		req := NewRequest(t, "POST", user2inboxurl)
		MakeRequest(t, req, 500)
	})
}
