// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegramPayload(t *testing.T) {
	tc := telegramConvertor{}

	t.Run("Correct webhook params", func(t *testing.T) {
		p := createTelegramPayloadHTML(`<a href=".">testMsg</a> <bad>`)
		assert.Equal(t, TelegramPayload{
			RichMessage: InputRichMessage{
				HTML: `<a href="." rel="nofollow">testMsg</a>`,
			},
		}, p)
	})

	t.Run("Line breaks are kept in rich messages", func(t *testing.T) {
		// Rich messages collapse bare newlines, so they must become <br>.
		p := createTelegramPayloadHTML("first line\r\nsecond line\nthird line")
		assert.Equal(t, "first line<br>second line<br>third line", p.RichMessage.HTML)
	})

	for _, testCase := range []struct {
		name string
		call func() (TelegramPayload, error)
		text string
	}{
		{
			"Create", func() (TelegramPayload, error) { return tc.Create(createTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] branch <a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a> created`,
		},
		{
			"Delete", func() (TelegramPayload, error) { return tc.Delete(deleteTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] branch <a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a> deleted`,
		},
		{
			"Fork", func() (TelegramPayload, error) { return tc.Fork(forkTestPayload()) },
			`test/repo2 is forked to <a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>`,
		},
		{
			"Push", func() (TelegramPayload, error) { return tc.Push(pushTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>:<a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a>] 2 new commits<br>[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1<br>[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1`,
		},
		{
			"IssueComment", func() (TelegramPayload, error) { return tc.IssueComment(issueCommentTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] New comment on issue <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a><br>more info needed`,
		},
		{
			"PullRequest", func() (TelegramPayload, error) { return tc.PullRequest(pullRequestTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Pull request opened: <a href="http://localhost:3000/test/repo/pulls/12" rel="nofollow">#12 Fix bug</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a><br>fixes bug #2`,
		},
		{
			"PullRequestComment", func() (TelegramPayload, error) { return tc.IssueComment(pullRequestCommentTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] New comment on pull request <a href="http://localhost:3000/test/repo/pulls/12" rel="nofollow">#12 Fix bug</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a><br>changes requested`,
		},
		{
			"Review", func() (TelegramPayload, error) {
				p := pullRequestTestPayload()
				p.Action = api.HookIssueReviewed
				return tc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
			},
			`[test/repo] Pull request review approved: #12 Fix bug<br>good job`,
		},
		{
			"Repository", func() (TelegramPayload, error) { return tc.Repository(repositoryTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Repository created`,
		},
		{
			"Package", func() (TelegramPayload, error) { return tc.Package(packageTestPayload()) },
			`Package created: <a href="http://localhost:3000/user1/-/packages/container/GiteaContainer/latest" rel="nofollow">GiteaContainer:latest</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`,
		},
		{
			"Release", func() (TelegramPayload, error) { return tc.Release(pullReleaseTestPayload()) },
			`[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Release created: <a href="http://localhost:3000/test/repo/releases/tag/v1.0" rel="nofollow">v1.0</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			pl, err := testCase.call()
			require.NoError(t, err)
			assert.Equal(t, testCase.text, pl.RichMessage.HTML)
		})
	}

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()
		for _, testCase := range []struct {
			action api.HookIssueAction
			text   string
		}{
			{api.HookIssueOpened, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Issue opened: <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a><br><br>issue body`},
			{api.HookIssueClosed, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Issue closed: <a href="http://localhost:3000/test/repo/issues/2" rel="nofollow">#2 crash</a> by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`},
		} {
			p.Action = testCase.action
			pl, err := tc.Issue(p)
			require.NoError(t, err)
			assert.Equal(t, testCase.text, pl.RichMessage.HTML)
		}
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()
		for _, testCase := range []struct {
			action api.HookWikiAction
			text   string
		}{
			{api.HookWikiCreated, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] New wiki page &#39;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#39; (Wiki change comment) by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`},
			{api.HookWikiEdited, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Wiki page &#39;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#39; edited (Wiki change comment) by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`},
			{api.HookWikiDeleted, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>] Wiki page &#39;<a href="http://localhost:3000/test/repo/wiki/index" rel="nofollow">index</a>&#39; deleted by <a href="https://try.gitea.io/user1" rel="nofollow">user1</a>`},
		} {
			p.Action = testCase.action
			pl, err := tc.Wiki(p)
			require.NoError(t, err)
			assert.Equal(t, testCase.text, pl.RichMessage.HTML)
		}
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
		URL:        "https://telegram.example.com/sendMessage",
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
	assert.Equal(t, "https://telegram.example.com/sendRichMessage", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body TelegramPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, `[<a href="http://localhost:3000/test/repo" rel="nofollow">test/repo</a>:<a href="http://localhost:3000/test/repo/src/test" rel="nofollow">test</a>] 2 new commits<br>[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1<br>[<a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778" rel="nofollow">2020558</a>] commit message - user1`, body.RichMessage.HTML)
}
