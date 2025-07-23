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
	"code.gitea.io/gitea/modules/test"
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
	defer test.MockVariableValue(&setting.Service.NoReplyAddress, "no-reply.com")()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.Equal(t, user.GetPlaceholderEmail(), convert.ToUser(db.DefaultContext, user, nil).Email)
	assert.Equal(t, user.Email, convert.ToUser(db.DefaultContext, user, user).Email)
}

func TestWebhookPayloadOptimization(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	var optimizedCommits []*api.PayloadCommit
	var optimizedHeadCommit *api.PayloadCommit

	// Create a test repository
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a webhook with file limit = 1
	webhook := &webhook_model.Webhook{
		RepoID:              repo.ID,
		URL:                 "http://example.com/webhook",
		HTTPMethod:          "POST",
		ContentType:         webhook_model.ContentTypeJSON,
		Secret:              "secret",
		IsActive:            true,
		Type:                webhook_module.GITEA,
		ExcludeFilesLimit:   1,
		ExcludeCommitsLimit: 0,
		HookEvent: &webhook_module.HookEvent{
			PushOnly: true,
		},
	}

	err := webhook.UpdateEvent()
	assert.NoError(t, err)
	err = webhook_model.CreateWebhook(db.DefaultContext, webhook)
	assert.NoError(t, err)
	assert.NotZero(t, webhook.ID)

	// Test payload optimization: should truncate to 1 file per field
	notifier := &webhookNotifier{}
	apiCommits := []*api.PayloadCommit{
		{
			ID:       "abc123",
			Message:  "Test commit",
			Added:    []string{"file1.txt", "file2.txt"},
			Removed:  []string{"oldfile.txt"},
			Modified: []string{"modified.txt"},
		},
		{
			ID:       "def456",
			Message:  "Another commit",
			Added:    []string{"file3.txt"},
			Removed:  []string{},
			Modified: []string{"file1.txt"},
		},
	}
	apiHeadCommit := &api.PayloadCommit{
		ID:       "def456",
		Message:  "Another commit",
		Added:    []string{"file3.txt"},
		Removed:  []string{},
		Modified: []string{"file1.txt"},
	}
	optimizedCommits, _ = notifier.applyWebhookPayloadOptimizations(db.DefaultContext, repo, apiCommits, apiHeadCommit)
	assert.Equal(t, []string{"file1.txt"}, optimizedCommits[0].Added)
	assert.Equal(t, []string{"oldfile.txt"}, optimizedCommits[0].Removed)
	assert.Equal(t, []string{"modified.txt"}, optimizedCommits[0].Modified)
	assert.Equal(t, []string{"file3.txt"}, optimizedCommits[1].Added)
	assert.Equal(t, []string{}, optimizedCommits[1].Removed)
	assert.Equal(t, []string{"file1.txt"}, optimizedCommits[1].Modified)

	_, optimizedHeadCommit = notifier.applyWebhookPayloadOptimizations(db.DefaultContext, repo, apiCommits, apiHeadCommit)
	assert.Equal(t, []string{"file3.txt"}, optimizedHeadCommit.Added)
	assert.Equal(t, []string{}, optimizedHeadCommit.Removed)
	assert.Equal(t, []string{"file1.txt"}, optimizedHeadCommit.Modified)

	// Test with commit limit = 1
	webhook.ExcludeFilesLimit = 0
	webhook.ExcludeCommitsLimit = 1
	err = webhook_model.UpdateWebhook(db.DefaultContext, webhook)
	assert.NoError(t, err)
	apiCommits = []*api.PayloadCommit{
		{
			ID:       "abc123",
			Message:  "Test commit",
			Added:    []string{"file1.txt", "file2.txt"},
			Removed:  []string{"oldfile.txt"},
			Modified: []string{"modified.txt"},
		},
		{
			ID:       "def456",
			Message:  "Another commit",
			Added:    []string{"file3.txt"},
			Removed:  []string{},
			Modified: []string{"file1.txt"},
		},
	}
	apiHeadCommit = &api.PayloadCommit{
		ID:       "def456",
		Message:  "Another commit",
		Added:    []string{"file3.txt"},
		Removed:  []string{},
		Modified: []string{"file1.txt"},
	}
	optimizedCommits, _ = notifier.applyWebhookPayloadOptimizations(db.DefaultContext, repo, apiCommits, apiHeadCommit)
	assert.Len(t, optimizedCommits, 1)
	assert.Equal(t, "abc123", optimizedCommits[0].ID)

	// Test with no limits (0 means unlimited)
	webhook.ExcludeFilesLimit = 0
	webhook.ExcludeCommitsLimit = 0
	err = webhook_model.UpdateWebhook(db.DefaultContext, webhook)
	assert.NoError(t, err)
	apiCommits = []*api.PayloadCommit{
		{
			ID:       "abc123",
			Message:  "Test commit",
			Added:    []string{"file1.txt", "file2.txt"},
			Removed:  []string{"oldfile.txt"},
			Modified: []string{"modified.txt"},
		},
		{
			ID:       "def456",
			Message:  "Another commit",
			Added:    []string{"file3.txt"},
			Removed:  []string{},
			Modified: []string{"file1.txt"},
		},
	}
	apiHeadCommit = &api.PayloadCommit{
		ID:       "def456",
		Message:  "Another commit",
		Added:    []string{"file3.txt"},
		Removed:  []string{},
		Modified: []string{"file1.txt"},
	}
	optimizedCommits, optimizedHeadCommit = notifier.applyWebhookPayloadOptimizations(db.DefaultContext, repo, apiCommits, apiHeadCommit)
	assert.Equal(t, []string{"file1.txt", "file2.txt"}, optimizedCommits[0].Added)
	assert.Equal(t, []string{"oldfile.txt"}, optimizedCommits[0].Removed)
	assert.Equal(t, []string{"modified.txt"}, optimizedCommits[0].Modified)
	assert.Equal(t, []string{"file3.txt"}, optimizedCommits[1].Added)
	assert.Equal(t, []string{}, optimizedCommits[1].Removed)
	assert.Equal(t, []string{"file1.txt"}, optimizedCommits[1].Modified)
	assert.Equal(t, []string{"file3.txt"}, optimizedHeadCommit.Added)
	assert.Equal(t, []string{}, optimizedHeadCommit.Removed)
	assert.Equal(t, []string{"file1.txt"}, optimizedHeadCommit.Modified)
}
