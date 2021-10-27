// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"github.com/go-fed/activity/streams"
	"github.com/go-fed/activity/streams/vocab"

	"github.com/stretchr/testify/assert"
)

func TestActivityPubPerson(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Federation.Enabled = true
		defer func() {
			setting.Federation.Enabled = false
		}()

		username := "user2"
		req := NewRequestf(t, "GET", fmt.Sprintf("/api/v1/activitypub/user/%s", username))
		resp := MakeRequest(t, req, http.StatusOK)
		assert.Contains(t, string(resp.Body.Bytes()), "@context")
		var m map[string]interface{}
		_ = json.Unmarshal(resp.Body.Bytes(), &m)

		var person vocab.ActivityStreamsPerson
		resolver, _ := streams.NewJSONResolver(func(c context.Context, p vocab.ActivityStreamsPerson) error {
			person = p
			return nil
		})
		ctx := context.Background()
		err := resolver.Resolve(ctx, m)
		assert.Equal(t, err, nil)
		assert.Equal(t, person.GetTypeName(), "Person")
		assert.Equal(t, person.GetActivityStreamsName().Begin().GetXMLSchemaString(), username)
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s$", username), person.GetJSONLDId().GetIRI().String())
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s/outbox$", username), person.GetActivityStreamsOutbox().GetIRI().String())
		assert.Regexp(t, fmt.Sprintf("activitypub/user/%s/inbox$", username), person.GetActivityStreamsInbox().GetIRI().String())
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
		assert.Contains(t, string(resp.Body.Bytes()), "GetUserByName")
	})
}
