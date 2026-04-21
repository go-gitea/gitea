// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	sender_service "code.gitea.io/gitea/services/mailer/sender"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailNewReleaseFiltersUnauthorizedWatchers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	defer test.MockVariableValue(&setting.MailService)()
	defer test.MockVariableValue(&setting.Domain)()
	defer test.MockVariableValue(&setting.AppName)()
	defer test.MockVariableValue(&setting.AppURL)()

	setting.MailService = &setting.Mailer{
		From:      "Gitea",
		FromEmail: "noreply@example.com",
	}
	setting.Domain = "example.com"
	setting.AppName = "Gitea"
	setting.AppURL = "https://example.com/"
	defer mockMailTemplates(string(tplNewReleaseMail), "{{.Subject}}", "<p>{{.Release.TagName}}</p>")()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.True(t, repo.IsPrivate)

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	unauthorized := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	assert.NoError(t, repo_model.WatchRepo(t.Context(), admin, repo, true))
	assert.NoError(t, repo_model.WatchRepo(t.Context(), unauthorized, repo, true))

	rel := unittest.AssertExistsAndLoadBean(t, &repo_model.Release{ID: 11})
	rel.Repo = nil
	rel.Publisher = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: rel.PublisherID})

	var sent []*sender_service.Message
	origSend := SendAsync
	SendAsync = func(msgs ...*sender_service.Message) {
		sent = append(sent, msgs...)
	}
	defer func() {
		SendAsync = origSend
	}()

	MailNewRelease(t.Context(), rel)

	require.Len(t, sent, 1)
	assert.Equal(t, admin.EmailTo(), sent[0].To)
	assert.NotEqual(t, unauthorized.EmailTo(), sent[0].To)
}
