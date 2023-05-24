// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"

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
	assert.NoError(t, unittest.PrepareTestDatabase())
	webhook := unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 1})
	tasks, err := webhook.History(0)
	assert.NoError(t, err)
	if assert.Len(t, tasks, 1) {
		assert.Equal(t, int64(1), tasks[0].ID)
	}

	webhook = unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 2})
	tasks, err = webhook.History(0)
	assert.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestWebhook_UpdateEvent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	webhook := unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 1})
	hookEvent := &webhook_module.HookEvent{
		PushOnly:       true,
		SendEverything: false,
		ChooseEvents:   false,
		HookEvents: webhook_module.HookEvents{
			Create:      false,
			Push:        true,
			PullRequest: false,
		},
	}
	webhook.HookEvent = hookEvent
	assert.NoError(t, webhook.UpdateEvent())
	assert.NotEmpty(t, webhook.Events)
	actualHookEvent := &webhook_module.HookEvent{}
	assert.NoError(t, json.Unmarshal([]byte(webhook.Events), actualHookEvent))
	assert.Equal(t, *hookEvent, *actualHookEvent)
}

func TestWebhook_EventsArray(t *testing.T) {
	assert.Equal(t, []string{
		"create", "delete", "fork", "push",
		"issues", "issue_assign", "issue_label", "issue_milestone", "issue_comment",
		"pull_request", "pull_request_assign", "pull_request_label", "pull_request_milestone",
		"pull_request_comment", "pull_request_review_approved", "pull_request_review_rejected",
		"pull_request_review_comment", "pull_request_sync", "wiki", "repository", "release",
		"package",
	},
		(&Webhook{
			HookEvent: &webhook_module.HookEvent{SendEverything: true},
		}).EventsArray(),
	)

	assert.Equal(t, []string{"push"},
		(&Webhook{
			HookEvent: &webhook_module.HookEvent{PushOnly: true},
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
	unittest.AssertNotExistsBean(t, hook)
	assert.NoError(t, CreateWebhook(db.DefaultContext, hook))
	unittest.AssertExistsAndLoadBean(t, hook)
}

func TestGetWebhookByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hook, err := GetWebhookByRepoID(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), hook.ID)

	_, err = GetWebhookByRepoID(unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetWebhookByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hook, err := GetWebhookByOwnerID(3, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), hook.ID)

	_, err = GetWebhookByOwnerID(unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetActiveWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := ListWebhooksByOpts(db.DefaultContext, &ListWebhookOptions{RepoID: 1, IsActive: util.OptionalBoolTrue})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := ListWebhooksByOpts(db.DefaultContext, &ListWebhookOptions{RepoID: 1})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 2) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.Equal(t, int64(2), hooks[1].ID)
	}
}

func TestGetActiveWebhooksByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := ListWebhooksByOpts(db.DefaultContext, &ListWebhookOptions{OwnerID: 3, IsActive: util.OptionalBoolTrue})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(3), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := ListWebhooksByOpts(db.DefaultContext, &ListWebhookOptions{OwnerID: 3})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(3), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestUpdateWebhook(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hook := unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 2})
	hook.IsActive = true
	hook.ContentType = ContentTypeForm
	unittest.AssertNotExistsBean(t, hook)
	assert.NoError(t, UpdateWebhook(hook))
	unittest.AssertExistsAndLoadBean(t, hook)
}

func TestDeleteWebhookByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 2, RepoID: 1})
	assert.NoError(t, DeleteWebhookByRepoID(1, 2))
	unittest.AssertNotExistsBean(t, &Webhook{ID: 2, RepoID: 1})

	err := DeleteWebhookByRepoID(unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestDeleteWebhookByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 3, OwnerID: 3})
	assert.NoError(t, DeleteWebhookByOwnerID(3, 3))
	unittest.AssertNotExistsBean(t, &Webhook{ID: 3, OwnerID: 3})

	err := DeleteWebhookByOwnerID(unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestHookTasks(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTasks, err := HookTasks(1, 1)
	assert.NoError(t, err)
	if assert.Len(t, hookTasks, 1) {
		assert.Equal(t, int64(1), hookTasks[0].ID)
	}

	hookTasks, err = HookTasks(unittest.NonexistentID, 1)
	assert.NoError(t, err)
	assert.Len(t, hookTasks, 0)
}

func TestCreateHookTask(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:    3,
		Payloader: &api.PushPayload{},
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func TestUpdateHookTask(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	hook := unittest.AssertExistsAndLoadBean(t, &HookTask{ID: 1})
	hook.PayloadContent = "new payload content"
	hook.IsDelivered = true
	unittest.AssertNotExistsBean(t, hook)
	assert.NoError(t, UpdateHookTask(hook))
	unittest.AssertExistsAndLoadBean(t, hook)
}

func TestCleanupHookTaskTable_PerWebhook_DeletesDelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:      3,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   timeutil.TimeStampNanoNow(),
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), PerWebhook, 168*time.Hour, 0))
	unittest.AssertNotExistsBean(t, hookTask)
}

func TestCleanupHookTaskTable_PerWebhook_LeavesUndelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: false,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), PerWebhook, 168*time.Hour, 0))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_PerWebhook_LeavesMostRecentTask(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   timeutil.TimeStampNanoNow(),
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), PerWebhook, 168*time.Hour, 1))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_DeletesDelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:      3,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   timeutil.TimeStampNano(time.Now().AddDate(0, 0, -8).UnixNano()),
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), OlderThan, 168*time.Hour, 0))
	unittest.AssertNotExistsBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_LeavesUndelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: false,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), OlderThan, 168*time.Hour, 0))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_LeavesTaskEarlierThanAgeToDelete(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   timeutil.TimeStampNano(time.Now().AddDate(0, 0, -6).UnixNano()),
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), OlderThan, 168*time.Hour, 0))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}
