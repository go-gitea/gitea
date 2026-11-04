// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"net/url"
	"testing"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDingTalkPayload(t *testing.T) {
	parseRealSingleURL := func(singleURL string) string {
		if u, err := url.Parse(singleURL); err == nil {
			assert.Equal(t, "dingtalk", u.Scheme)
			assert.Equal(t, "dingtalkclient", u.Host)
			assert.Equal(t, "/page/link", u.Path)
			return u.Query().Get("url")
		}
		return ""
	}
	dc := dingtalkConvertor{}
	assertPayload := func(t *testing.T, pl DingtalkPayload, text, title, singleTitle, singleURL string) {
		assert.Equal(t, text, pl.ActionCard.Text)
		assert.Equal(t, title, pl.ActionCard.Title)
		assert.Equal(t, singleTitle, pl.ActionCard.SingleTitle)
		assert.Equal(t, singleURL, parseRealSingleURL(pl.ActionCard.SingleURL))
	}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := dc.Create(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] branch test created", "[test/repo] branch test created", "view ref test", "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := dc.Delete(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] branch test deleted", "[test/repo] branch test deleted", "view ref test", "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := dc.Fork(p)
		require.NoError(t, err)

		assertPayload(t, pl, "test/repo2 is forked to test/repo", "test/repo2 is forked to test/repo", "view forked repo test/repo", "http://localhost:3000/test/repo")
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", "[test/repo:test] 2 new commits", "view commits", "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := dc.Issue(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Issue opened: #2 crash by user1\r\n\r\nissue body", "#2 crash", "view issue", "http://localhost:3000/test/repo/issues/2")

		p.Action = api.HookIssueClosed
		pl, err = dc.Issue(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Issue closed: #2 crash by user1", "#2 crash", "view issue", "http://localhost:3000/test/repo/issues/2")
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] New comment on issue #2 crash by user1\r\n\r\nmore info needed", "#2 crash", "view issue comment", "http://localhost:3000/test/repo/issues/2#issuecomment-4")
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := dc.PullRequest(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Pull request opened: #12 Fix bug by user1\r\n\r\nfixes bug #2", "#12 Fix bug", "view pull request", "http://localhost:3000/test/repo/pulls/12")
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] New comment on pull request #12 Fix bug by user1\r\n\r\nchanges requested", "#12 Fix bug", "view issue comment", "http://localhost:3000/test/repo/pulls/12#issuecomment-4")
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := dc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", "[test/repo] Pull request review approved : #12 Fix bug", "view pull request", "http://localhost:3000/test/repo/pulls/12")
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := dc.Repository(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Repository created", "[test/repo] Repository created", "view repository", "http://localhost:3000/test/repo")
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := dc.Package(p)
		require.NoError(t, err)

		assertPayload(t, pl, "Package created: GiteaContainer:latest by user1", "Package created: GiteaContainer:latest by user1", "view package", "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest")
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := dc.Wiki(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] New wiki page 'index' (Wiki change comment) by user1", "[test/repo] New wiki page 'index' (Wiki change comment) by user1", "view wiki", "http://localhost:3000/test/repo/wiki/index")

		p.Action = api.HookWikiEdited
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Wiki page 'index' edited (Wiki change comment) by user1", "[test/repo] Wiki page 'index' edited (Wiki change comment) by user1", "view wiki", "http://localhost:3000/test/repo/wiki/index")

		p.Action = api.HookWikiDeleted
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Wiki page 'index' deleted by user1", "[test/repo] Wiki page 'index' deleted by user1", "view wiki", "http://localhost:3000/test/repo/wiki/index")
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := dc.Release(p)
		require.NoError(t, err)

		assertPayload(t, pl, "[test/repo] Release created: v1.0 by user1", "[test/repo] Release created: v1.0 by user1", "view release", "http://localhost:3000/test/repo/releases/tag/v1.0")
	})
}

func TestDingTalkJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.DINGTALK,
		URL:        "https://dingtalk.example.com/",
		Meta:       ``,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newDingtalkRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://dingtalk.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body DingtalkPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", body.ActionCard.Text)
}
