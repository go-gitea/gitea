// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPILockIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("Lock", func(t *testing.T) {
		issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		assert.False(t, issueBefore.IsLocked)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", owner.Name, repo.Name, issueBefore.Index)
		hook := prepareIssueLockWebhook(t, repo.ID)

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

		// check lock issue
		req := NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		issueAfter := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		assert.True(t, issueAfter.IsLocked)
		assertIssueLockWebhookTask(t, hook, api.HookIssueLocked)

		// check with other user
		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		session34 := loginUser(t, user34.Name)
		token34 := getTokenForLoggedInUser(t, session34, auth_model.AccessTokenScopeAll)
		req = NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token34)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("Unlock", func(t *testing.T) {
		issueBefore := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issueBefore.RepoID})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", owner.Name, repo.Name, issueBefore.Index)
		hook := prepareIssueLockWebhook(t, repo.ID)

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

		lockReq := NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token)
		MakeRequest(t, lockReq, http.StatusNoContent)
		require.NoError(t, db.TruncateBeans(t.Context(), &webhook_model.HookTask{}))

		// check unlock issue
		req := NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)
		issueAfter := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
		assert.False(t, issueAfter.IsLocked)
		assertIssueLockWebhookTask(t, hook, api.HookIssueUnlocked)

		// check with other user
		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		session34 := loginUser(t, user34.Name)
		token34 := getTokenForLoggedInUser(t, session34, auth_model.AccessTokenScopeAll)
		req = NewRequest(t, "DELETE", urlStr).AddTokenAuth(token34)
		MakeRequest(t, req, http.StatusForbidden)
	})
}

func prepareIssueLockWebhook(t *testing.T, repoID int64) *webhook_model.Webhook {
	t.Helper()

	require.NoError(t, db.TruncateBeans(t.Context(), &webhook_model.Webhook{}, &webhook_model.HookTask{}))
	hook := &webhook_model.Webhook{
		RepoID:      repoID,
		URL:         "http://example.com/gitea-test-issue-lock",
		ContentType: webhook_model.ContentTypeJSON,
		IsActive:    true,
		Type:        webhook_module.GITEA,
		HookEvent: &webhook_module.HookEvent{
			ChooseEvents: true,
			HookEvents: webhook_module.HookEvents{
				webhook_module.HookEventIssues: true,
			},
		},
	}
	require.NoError(t, hook.UpdateEvent())
	require.NoError(t, webhook_model.CreateWebhook(t.Context(), hook))
	return hook
}

func assertIssueLockWebhookTask(t *testing.T, hook *webhook_model.Webhook, action api.HookIssueAction) {
	t.Helper()

	task := unittest.AssertExistsAndLoadBean(t, &webhook_model.HookTask{HookID: hook.ID, EventType: webhook_module.HookEventIssues})
	var payload api.IssuePayload
	require.NoError(t, json.Unmarshal([]byte(task.PayloadContent), &payload))
	assert.Equal(t, action, payload.Action)
	assert.Equal(t, hook.RepoID, payload.Repository.ID)
	assert.Equal(t, "issue1", payload.Issue.Title)
}
