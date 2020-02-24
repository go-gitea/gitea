// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestWebhook_GetSlackHook(t *testing.T) {
	w := &models.Webhook{
		Meta: `{"channel": "foo", "username": "username", "color": "blue"}`,
	}
	slackHook := GetSlackHook(w)
	assert.Equal(t, *slackHook, SlackMeta{
		Channel:  "foo",
		Username: "username",
		Color:    "blue",
	})
}

func TestPrepareWebhooks(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	hookTasks := []*models.HookTask{
		{RepoID: repo.ID, HookID: 1, EventType: models.HookEventPush},
	}
	for _, hookTask := range hookTasks {
		models.AssertNotExistsBean(t, hookTask)
	}
	assert.NoError(t, PrepareWebhooks(repo, models.HookEventPush, &api.PushPayload{}))
	for _, hookTask := range hookTasks {
		models.AssertExistsAndLoadBean(t, hookTask)
	}
}

func TestPrepareWebhooksBranchFilterMatch(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	hookTasks := []*models.HookTask{
		{RepoID: repo.ID, HookID: 4, EventType: models.HookEventPush},
	}
	for _, hookTask := range hookTasks {
		models.AssertNotExistsBean(t, hookTask)
	}
	// this test also ensures that * doesn't handle / in any special way (like shell would)
	assert.NoError(t, PrepareWebhooks(repo, models.HookEventPush, &api.PushPayload{Ref: "refs/heads/feature/7791"}))
	for _, hookTask := range hookTasks {
		models.AssertExistsAndLoadBean(t, hookTask)
	}
}

func TestPrepareWebhooksBranchFilterNoMatch(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	hookTasks := []*models.HookTask{
		{RepoID: repo.ID, HookID: 4, EventType: models.HookEventPush},
	}
	for _, hookTask := range hookTasks {
		models.AssertNotExistsBean(t, hookTask)
	}
	assert.NoError(t, PrepareWebhooks(repo, models.HookEventPush, &api.PushPayload{Ref: "refs/heads/fix_weird_bug"}))

	for _, hookTask := range hookTasks {
		models.AssertNotExistsBean(t, hookTask)
	}
}

// TODO TestHookTask_deliver

// TODO TestDeliverHooks
