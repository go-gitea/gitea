// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/timeutil"
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
	tasks, err := webhook.History(db.DefaultContext, 0)
	assert.NoError(t, err)
	if assert.Len(t, tasks, 3) {
		assert.Equal(t, int64(3), tasks[0].ID)
		assert.Equal(t, int64(2), tasks[1].ID)
		assert.Equal(t, int64(1), tasks[2].ID)
	}

	webhook = unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 2})
	tasks, err = webhook.History(db.DefaultContext, 0)
	assert.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestWebhook_UpdateEvent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	webhook := unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 1})
	hookEvent := &webhook_module.HookEvent{
		PushOnly:       true,
		SendEverything: false,
		ChooseEvents:   false,
		HookEvents: webhook_module.HookEvents{
			webhook_module.HookEventCreate:      false,
			webhook_module.HookEventPush:        true,
			webhook_module.HookEventPullRequest: false,
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
	assert.EqualValues(t, []string{
		"create", "delete", "fork", "push",
		"issues", "issue_assign", "issue_label", "issue_milestone", "issue_comment",
		"pull_request", "pull_request_assign", "pull_request_label", "pull_request_milestone",
		"pull_request_comment", "pull_request_review_approved", "pull_request_review_rejected",
		"pull_request_review_comment", "pull_request_sync", "pull_request_review_request", "wiki", "repository", "release",
		"package", "status", "workflow_job",
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
	hook, err := GetWebhookByRepoID(db.DefaultContext, 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), hook.ID)

	_, err = GetWebhookByRepoID(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetWebhookByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hook, err := GetWebhookByOwnerID(db.DefaultContext, 3, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), hook.ID)

	_, err = GetWebhookByOwnerID(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetActiveWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := db.Find[Webhook](db.DefaultContext, ListWebhookOptions{RepoID: 1, IsActive: optional.Some(true)})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := db.Find[Webhook](db.DefaultContext, ListWebhookOptions{RepoID: 1})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 2) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.Equal(t, int64(2), hooks[1].ID)
	}
}

func TestGetActiveWebhooksByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := db.Find[Webhook](db.DefaultContext, ListWebhookOptions{OwnerID: 3, IsActive: optional.Some(true)})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(3), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hooks, err := db.Find[Webhook](db.DefaultContext, ListWebhookOptions{OwnerID: 3})
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
	assert.NoError(t, UpdateWebhook(db.DefaultContext, hook))
	unittest.AssertExistsAndLoadBean(t, hook)
}

func TestDeleteWebhookByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 2, RepoID: 1})
	assert.NoError(t, DeleteWebhookByRepoID(db.DefaultContext, 1, 2))
	unittest.AssertNotExistsBean(t, &Webhook{ID: 2, RepoID: 1})

	err := DeleteWebhookByRepoID(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestDeleteWebhookByOwnerID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	unittest.AssertExistsAndLoadBean(t, &Webhook{ID: 3, OwnerID: 3})
	assert.NoError(t, DeleteWebhookByOwnerID(db.DefaultContext, 3, 3))
	unittest.AssertNotExistsBean(t, &Webhook{ID: 3, OwnerID: 3})

	err := DeleteWebhookByOwnerID(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestHookTasks(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTasks, err := HookTasks(db.DefaultContext, 1, 1)
	assert.NoError(t, err)
	if assert.Len(t, hookTasks, 3) {
		assert.Equal(t, int64(3), hookTasks[0].ID)
		assert.Equal(t, int64(2), hookTasks[1].ID)
		assert.Equal(t, int64(1), hookTasks[2].ID)
	}

	hookTasks, err = HookTasks(db.DefaultContext, unittest.NonexistentID, 1)
	assert.NoError(t, err)
	assert.Empty(t, hookTasks)
}

func TestCreateHookTask(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:         3,
		PayloadVersion: 2,
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
	assert.NoError(t, UpdateHookTask(db.DefaultContext, hook))
	unittest.AssertExistsAndLoadBean(t, hook)
}

func TestCleanupHookTaskTable_PerWebhook_DeletesDelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:         3,
		IsDelivered:    true,
		Delivered:      timeutil.TimeStampNanoNow(),
		PayloadVersion: 2,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(t.Context(), PerWebhook, 168*time.Hour, 0))
	unittest.AssertNotExistsBean(t, hookTask)
}

func TestCleanupHookTaskTable_PerWebhook_LeavesUndelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:         4,
		IsDelivered:    false,
		PayloadVersion: 2,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(t.Context(), PerWebhook, 168*time.Hour, 0))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_PerWebhook_LeavesMostRecentTask(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:         4,
		IsDelivered:    true,
		Delivered:      timeutil.TimeStampNanoNow(),
		PayloadVersion: 2,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(t.Context(), PerWebhook, 168*time.Hour, 1))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_DeletesDelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:         3,
		IsDelivered:    true,
		Delivered:      timeutil.TimeStampNano(time.Now().AddDate(0, 0, -8).UnixNano()),
		PayloadVersion: 2,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(t.Context(), OlderThan, 168*time.Hour, 0))
	unittest.AssertNotExistsBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_LeavesUndelivered(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:         4,
		IsDelivered:    false,
		PayloadVersion: 2,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(t.Context(), OlderThan, 168*time.Hour, 0))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_LeavesTaskEarlierThanAgeToDelete(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	hookTask := &HookTask{
		HookID:         4,
		IsDelivered:    true,
		Delivered:      timeutil.TimeStampNano(time.Now().AddDate(0, 0, -6).UnixNano()),
		PayloadVersion: 2,
	}
	unittest.AssertNotExistsBean(t, hookTask)
	_, err := CreateHookTask(db.DefaultContext, hookTask)
	assert.NoError(t, err)
	unittest.AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(t.Context(), OlderThan, 168*time.Hour, 0))
	unittest.AssertExistsAndLoadBean(t, hookTask)
}
