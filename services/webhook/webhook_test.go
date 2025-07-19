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

	// Create a test repository
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a webhook with payload optimization enabled
	webhook := &webhook_model.Webhook{
		RepoID:         repo.ID,
		URL:            "http://example.com/webhook",
		HTTPMethod:     "POST",
		ContentType:    webhook_model.ContentTypeJSON,
		Secret:         "secret",
		IsActive:       true,
		Type:           webhook_module.GITEA,
		ExcludeFiles:   true,
		ExcludeCommits: false,
		HookEvent: &webhook_module.HookEvent{
			PushOnly: true,
		},
	}

	err := webhook_model.CreateWebhook(db.DefaultContext, webhook)
	assert.NoError(t, err)

	// Create test commits with file information
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

	// Test payload optimization
	notifier := &webhookNotifier{}
	optimizedCommits, optimizedHeadCommit := notifier.applyWebhookPayloadOptimizations(db.DefaultContext, repo, apiCommits, apiHeadCommit)

	// Verify that file information was removed when ExcludeFiles is true
	assert.Nil(t, optimizedCommits[0].Added)
	assert.Nil(t, optimizedCommits[0].Removed)
	assert.Nil(t, optimizedCommits[0].Modified)
	assert.Nil(t, optimizedCommits[1].Added)
	assert.Nil(t, optimizedCommits[1].Removed)
	assert.Nil(t, optimizedCommits[1].Modified)
	assert.Nil(t, optimizedHeadCommit.Added)
	assert.Nil(t, optimizedHeadCommit.Removed)
	assert.Nil(t, optimizedHeadCommit.Modified)

	// Test with ExcludeCommits enabled
	webhook.ExcludeFiles = false
	webhook.ExcludeCommits = true
	err = webhook_model.UpdateWebhook(db.DefaultContext, webhook)
	assert.NoError(t, err)

	optimizedCommits, optimizedHeadCommit = notifier.applyWebhookPayloadOptimizations(db.DefaultContext, repo, apiCommits, apiHeadCommit)

	// Verify that commits and head_commit were excluded
	assert.Nil(t, optimizedCommits)
	assert.Nil(t, optimizedHeadCommit)
}
