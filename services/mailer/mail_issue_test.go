// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"

	"gitea.dev/models/activities"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	sender_service "gitea.dev/services/mailer/sender"

	"github.com/stretchr/testify/assert"
)

func TestMailNewIssueAndPullRequest(t *testing.T) {
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
	defer mockMailTemplates(string("repo/issue/new"), "{{.Subject}}", "<p>Issue</p>")()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	watcher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	didSend := false
	origSend := SendAsync
	SendAsync = func(msgs ...*sender_service.Message) {
		for _, msg := range msgs {
			if msg.To == watcher.Email {
				didSend = true
			}
		}
	}
	defer func() {
		SendAsync = origSend
	}()

	iss := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3})

	assert.NoError(t, issues_model.RemoveIssueWatchersByRepoID(t.Context(), watcher.ID, repo.ID))
	assert.NoError(t, repo_model.WatchRepo(t.Context(), watcher, repo, true))
	assert.NoError(t, repo_model.WatchRepoOptions(t.Context(), watcher, repo, repo_model.WatchOptions{
		PullRequests: false,
		Issues:       false,
		Releases:     true,
	}))

	var mentions []*user_model.User
	assert.NoError(t, MailParticipants(t.Context(), iss, doer, activities.ActionCreateIssue, mentions))
	assert.False(t, didSend)

	didSend = false
	assert.NoError(t, MailParticipants(t.Context(), pr, doer, activities.ActionCreatePullRequest, mentions))
	assert.False(t, didSend)

	assert.NoError(t, repo_model.WatchRepoOptions(t.Context(), watcher, repo, repo_model.WatchOptions{
		PullRequests: true,
		Issues:       true,
		Releases:     true,
	}))
	didSend = false
	assert.NoError(t, MailParticipants(t.Context(), pr, doer, activities.ActionCreatePullRequest, mentions))
	assert.True(t, didSend)

	didSend = false
	assert.NoError(t, MailParticipants(t.Context(), iss, doer, activities.ActionCreateIssue, mentions))
	assert.True(t, didSend)
}
