// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	"code.gitea.io/gitea/services/convert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhook_GetSlackHook(t *testing.T) {
	w := &webhook_model.Webhook{
		Meta: `{"channel": "foo", "username": "username", "color": "blue"}`,
	}
	slackHook := GetSlackHook(w)
	assert.Equal(t, SlackMeta{
		Channel:  "foo",
		Username: "username",
		Color:    "blue",
	}, *slackHook)
}

func TestPrepareWebhooks(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	hookTasks := []*webhook_model.HookTask{
		{HookID: 1, EventType: webhook_module.HookEventPush},
	}
	for _, hookTask := range hookTasks {
		unittest.AssertNotExistsBean(t, hookTask)
	}
	assert.NoError(t, PrepareWebhooks(db.DefaultContext, EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{Commits: []*api.PayloadCommit{{}}}))
	for _, hookTask := range hookTasks {
		unittest.AssertExistsAndLoadBean(t, hookTask)
	}
}

func TestPrepareWebhooksBranchFilterMatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	hookTasks := []*webhook_model.HookTask{
		{HookID: 4, EventType: webhook_module.HookEventPush},
	}
	for _, hookTask := range hookTasks {
		unittest.AssertNotExistsBean(t, hookTask)
	}
	// this test also ensures that * doesn't handle / in any special way (like shell would)
	assert.NoError(t, PrepareWebhooks(db.DefaultContext, EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{Ref: "refs/heads/feature/7791", Commits: []*api.PayloadCommit{{}}}))
	for _, hookTask := range hookTasks {
		unittest.AssertExistsAndLoadBean(t, hookTask)
	}
}

func TestPrepareWebhooksBranchFilterNoMatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	hookTasks := []*webhook_model.HookTask{
		{HookID: 4, EventType: webhook_module.HookEventPush},
	}
	for _, hookTask := range hookTasks {
		unittest.AssertNotExistsBean(t, hookTask)
	}
	assert.NoError(t, PrepareWebhooks(db.DefaultContext, EventSource{Repository: repo}, webhook_module.HookEventPush, &api.PushPayload{Ref: "refs/heads/fix_weird_bug"}))

	for _, hookTask := range hookTasks {
		unittest.AssertNotExistsBean(t, hookTask)
	}
}

func TestWebhookUserMail(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	setting.Service.NoReplyAddress = "no-reply.com"
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.Equal(t, user.GetPlaceholderEmail(), convert.ToUser(db.DefaultContext, user, nil).Email)
	assert.Equal(t, user.Email, convert.ToUser(db.DefaultContext, user, user).Email)
}
