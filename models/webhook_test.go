// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestHookContentType_Name(t *testing.T) {
	assert.Equal(t, "json", ContentTypeJSON.Name())
	assert.Equal(t, "form", ContentTypeForm.Name())
}

func TestIsValidHookContentType(t *testing.T) {
	assert.True(t, IsValidHookContentType("json"))
	assert.True(t, IsValidHookContentType("form"))
	assert.False(t, IsValidHookContentType("invalid"))
}

func TestWebhook_History(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	webhook := AssertExistsAndLoadBean(t, &Webhook{ID: 1}).(*Webhook)
	tasks, err := webhook.History(0)
	assert.NoError(t, err)
	if assert.Len(t, tasks, 1) {
		assert.Equal(t, int64(1), tasks[0].ID)
	}

	webhook = AssertExistsAndLoadBean(t, &Webhook{ID: 2}).(*Webhook)
	tasks, err = webhook.History(0)
	assert.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestWebhook_UpdateEvent(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	webhook := AssertExistsAndLoadBean(t, &Webhook{ID: 1}).(*Webhook)
	hookEvent := &HookEvent{
		PushOnly:       true,
		SendEverything: false,
		ChooseEvents:   false,
		HookEvents: HookEvents{
			Create:      false,
			Push:        true,
			PullRequest: false,
		},
	}
	webhook.HookEvent = hookEvent
	assert.NoError(t, webhook.UpdateEvent())
	assert.NotEmpty(t, webhook.Events)
	actualHookEvent := &HookEvent{}
	assert.NoError(t, json.Unmarshal([]byte(webhook.Events), actualHookEvent))
	assert.Equal(t, *hookEvent, *actualHookEvent)
}

func TestWebhook_EventsArray(t *testing.T) {
	assert.Equal(t, []string{"create", "delete", "fork", "push", "issues", "issue_comment", "pull_request", "repository", "release"},
		(&Webhook{
			HookEvent: &HookEvent{SendEverything: true},
		}).EventsArray(),
	)

	assert.Equal(t, []string{"push"},
		(&Webhook{
			HookEvent: &HookEvent{PushOnly: true},
		}).EventsArray(),
	)
}

func TestCreateWebhook(t *testing.T) {
	hook := &Webhook{
		RepoID:      3,
		URL:         "www.example.com/unit_test",
		ContentType: ContentTypeJSON,
		Events:      `{"push_only":false,"send_everything":false,"choose_events":false,"events":{"create":false,"push":true,"pull_request":true}}`,
	}
	AssertNotExistsBean(t, hook)
	assert.NoError(t, CreateWebhook(hook))
	AssertExistsAndLoadBean(t, hook)
}

func TestGetWebhookByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hook, err := GetWebhookByRepoID(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), hook.ID)

	_, err = GetWebhookByRepoID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetWebhookByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hook, err := GetWebhookByOrgID(3, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), hook.ID)

	_, err = GetWebhookByOrgID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetActiveWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetActiveWebhooksByRepoID(1)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetWebhooksByRepoID(1)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 2) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.Equal(t, int64(2), hooks[1].ID)
	}
}

func TestGetActiveWebhooksByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetActiveWebhooksByOrgID(3)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(3), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetWebhooksByOrgID(3)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(3), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}

}

func TestUpdateWebhook(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hook := AssertExistsAndLoadBean(t, &Webhook{ID: 2}).(*Webhook)
	hook.IsActive = true
	hook.ContentType = ContentTypeForm
	AssertNotExistsBean(t, hook)
	assert.NoError(t, UpdateWebhook(hook))
	AssertExistsAndLoadBean(t, hook)
}

func TestDeleteWebhookByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	AssertExistsAndLoadBean(t, &Webhook{ID: 2, RepoID: 1})
	assert.NoError(t, DeleteWebhookByRepoID(1, 2))
	AssertNotExistsBean(t, &Webhook{ID: 2, RepoID: 1})

	err := DeleteWebhookByRepoID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestDeleteWebhookByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	AssertExistsAndLoadBean(t, &Webhook{ID: 3, OrgID: 3})
	assert.NoError(t, DeleteWebhookByOrgID(3, 3))
	AssertNotExistsBean(t, &Webhook{ID: 3, OrgID: 3})

	err := DeleteWebhookByOrgID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestToHookTaskType(t *testing.T) {
	assert.Equal(t, GOGS, ToHookTaskType("gogs"))
	assert.Equal(t, SLACK, ToHookTaskType("slack"))
	assert.Equal(t, GITEA, ToHookTaskType("gitea"))
	assert.Equal(t, TELEGRAM, ToHookTaskType("telegram"))
}

func TestHookTaskType_Name(t *testing.T) {
	assert.Equal(t, "gogs", GOGS.Name())
	assert.Equal(t, "slack", SLACK.Name())
	assert.Equal(t, "gitea", GITEA.Name())
	assert.Equal(t, "telegram", TELEGRAM.Name())
}

func TestIsValidHookTaskType(t *testing.T) {
	assert.True(t, IsValidHookTaskType("gogs"))
	assert.True(t, IsValidHookTaskType("slack"))
	assert.True(t, IsValidHookTaskType("gitea"))
	assert.True(t, IsValidHookTaskType("telegram"))
	assert.False(t, IsValidHookTaskType("invalid"))
}

func TestHookTasks(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTasks, err := HookTasks(1, 1)
	assert.NoError(t, err)
	if assert.Len(t, hookTasks, 1) {
		assert.Equal(t, int64(1), hookTasks[0].ID)
	}

	hookTasks, err = HookTasks(NonexistentID, 1)
	assert.NoError(t, err)
	assert.Len(t, hookTasks, 0)
}

func TestCreateHookTask(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:    3,
		HookID:    3,
		Type:      GITEA,
		URL:       "http://www.example.com/unit_test",
		Payloader: &api.PushPayload{},
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)
}

func TestUpdateHookTask(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	hook := AssertExistsAndLoadBean(t, &HookTask{ID: 1}).(*HookTask)
	hook.PayloadContent = "new payload content"
	hook.DeliveredString = "new delivered string"
	hook.IsDelivered = true
	AssertNotExistsBean(t, hook)
	assert.NoError(t, UpdateHookTask(hook))
	AssertExistsAndLoadBean(t, hook)
}
