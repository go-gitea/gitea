// Copyright 2021 The Gitea Authors. All rights reserved.
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

func TestFeishuPayload(t *testing.T) {
	fc := feishuConvertor{}
	for _, test := range []struct {
		name    string
		convert func() (FeishuPayload, error)
		text    string
	}{
		{
			"Create",
			func() (FeishuPayload, error) { return fc.Create(createTestPayload()) },
			`[test/repo] branch test created`,
		},
		{
			"Delete",
			func() (FeishuPayload, error) { return fc.Delete(deleteTestPayload()) },
			`[test/repo] branch test deleted`,
		},
		{
			"Fork",
			func() (FeishuPayload, error) { return fc.Fork(forkTestPayload()) },
			`test/repo2 is forked to test/repo`,
		},
		{
			"Push",
			func() (FeishuPayload, error) { return fc.Push(pushTestPayload()) },
			"[test/repo:test] \r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1",
		},
		{
			"IssueComment",
			func() (FeishuPayload, error) { return fc.IssueComment(issueCommentTestPayload()) },
			"[Comment-test/repo #2]: created\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\n\nmore info needed",
		},
		{
			"PullRequest",
			func() (FeishuPayload, error) { return fc.PullRequest(pullRequestTestPayload()) },
			"[PullRequest-test/repo #12]: opened\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\nAssignees: user1\n\nfixes bug #2",
		},
		{
			"PullRequestComment",
			func() (FeishuPayload, error) { return fc.IssueComment(pullRequestCommentTestPayload()) },
			"[Comment-test/repo #12]: created\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\n\nchanges requested",
		},
		{
			"Repository",
			func() (FeishuPayload, error) { return fc.Repository(repositoryTestPayload()) },
			"[test/repo] Repository created",
		},
		{
			"Package",
			func() (FeishuPayload, error) { return fc.Package(packageTestPayload()) },
			"Package created: GiteaContainer:latest by user1",
		},
		{
			"Release",
			func() (FeishuPayload, error) { return fc.Release(pullReleaseTestPayload()) },
			"[test/repo] Release created: v1.0 by user1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pl, err := test.convert()
			require.NoError(t, err)
			assert.Equal(t, test.text, pl.Content.Text)
		})
	}

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := fc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[Issue-test/repo #2]: opened\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.Content.Text)

		p.Action = api.HookIssueClosed
		pl, err = fc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[Issue-test/repo #2]: closed\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.Content.Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := fc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", pl.Content.Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New wiki page 'index' (Wiki change comment) by user1", pl.Content.Text)

		p.Action = api.HookWikiEdited
		pl, err = fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page 'index' edited (Wiki change comment) by user1", pl.Content.Text)

		p.Action = api.HookWikiDeleted
		pl, err = fc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page 'index' deleted by user1", pl.Content.Text)
	})
}

func TestFeishuJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.FEISHU,
		URL:        "https://feishu.example.com/",
		Meta:       `{}`,
		HTTPMethod: "POST",
		Secret:     "secret",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newFeishuRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://feishu.example.com/", req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body FeishuPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, "[test/repo:test] \r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", body.Content.Text)
	assert.Equal(t, feishuGenSign(hook.Secret, body.Timestamp), body.Sign)

	// a separate sign test, the result is generated by official python code, so the algo must be correct
	assert.Equal(t, "rWZ84lcag1x9aBFhn1gtV4ZN+4gme3pilfQNMk86vKg=", feishuGenSign("a", 1))
}
