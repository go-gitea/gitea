// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package webhook

package webhook

import (
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTeamCityPayload(t *testing.T) {
	t.Run("Payload isn't altered.", func(t *testing.T) {
		p := createTestPayload()

		pl, err := GetTeamCityPayload(p, webhook_model.HookEventPush, "")
		require.NoError(t, err)
		require.Equal(t, p, pl)
	})
}

func TestWebhook_GetTeamCityHook(t *testing.T) {
	t.Run("GetTeamCityHook", func(t *testing.T) {
		w := &webhook_model.Webhook{
			Meta: `{"host_url": "http://localhost.com", "auth_token": "testToken", "vcs_root_id" :"fooVCS"}`,
		}

		teamcityHook := GetTeamCityHook(w)
		assert.Equal(t, *teamcityHook, TeamCityMeta{
			HostURL:   "http://localhost.com",
			AuthToken: "testToken",
			VcsRootID: "fooVCS",
		})
	})
}
