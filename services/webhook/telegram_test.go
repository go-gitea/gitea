// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramPayload(t *testing.T) {
	tc := telegramConvertor{}

	t.Run("Correct webhook params", func(t *testing.T) {
		p := createTelegramPayloadHTML(`<a href=".">testMsg</a> <bad>`)
		assert.Equal(t, TelegramPayload{
			Message:           `<a href="." rel="nofollow">testMsg</a>`,
			ParseMode:         "HTML",
			DisableWebPreview: true,
		}, p)
	})

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := tc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] branch <a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a> created`, pl.Message)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := tc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] branch <a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a> deleted`, pl.Message)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := tc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, `test/repo2 is forked to <a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>`, pl.Message)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := tc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>:<a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a>] 2 new commits
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1`, pl.Message)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := tc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Issue opened: <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>

issue body`, pl.Message)

		p.Action = api.HookIssueClosed
		pl, err = tc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Issue closed: <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`, pl.Message)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := tc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] New comment on issue <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>
more info needed`, pl.Message)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := tc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Pull request opened: <a href="http://localhost:3000/test/repo/pulls/12" rel="nofollow">#12 Fix bug</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>
fixes bug #2`, pl.Message)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := tc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] New comment on pull request <a href="http://localhost:3000/test/repo/pulls/12" rel="nofollow">#12 Fix bug</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>
changes requested`, pl.Message)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := tc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, `[test/repo] Pull request review approved: #12 Fix bug
good job`, pl.Message)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := tc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Repository created`, pl.Message)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := tc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, `Package created: <a href="http://localhost:3000/user1/-/packages/container/GiteaContainer/latest" rel="nofollow">GiteaContainer:latest</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`, pl.Message)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := tc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] New wiki page &#39;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#39; (Wiki change comment) by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`, pl.Message)

		p.Action = api.HookWikiEdited
		pl, err = tc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Wiki page &#39;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#39; edited (Wiki change comment) by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`, pl.Message)

		p.Action = api.HookWikiDeleted
		pl, err = tc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Wiki page &#39;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#39; deleted by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`, pl.Message)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := tc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Release created: <a href="http://localhost:3000/test/repo/releases/tag/v1.0" rel="nofollow">v1.0</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`, pl.Message)
	})
}

func TestTelegramJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.TELEGRAM,
		URL:        "https://telegram.example.com/",
		Meta:       ``,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newTelegramRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://telegram.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body TelegramPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>:<a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a>] 2 new commits
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1
[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1`, body.Message)
}
