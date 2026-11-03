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

func TestSlackPayload(t *testing.T) {
	sc := slackConvertor{}

	for _, testCase := range []struct {
		name        string
		call        func() (SlackPayload, error)
		text        string
		attachments bool
	}{
		{
			"Create", func() (SlackPayload, error) { return sc.Create(createTestPayload()) },
			"[<http://localhost:3000/test/repo|test/repo>:<http://localhost:3000/test/repo/src/branch/test|test>] branch created by user1", false,
		},
		{
			"Delete", func() (SlackPayload, error) { return sc.Delete(deleteTestPayload()) },
			"[<http://localhost:3000/test/repo|test/repo>:test] branch deleted by user1", false,
		},
		{
			"Fork", func() (SlackPayload, error) { return sc.Fork(forkTestPayload()) },
			"<http://localhost:3000/test/repo2|test/repo2> is forked to <http://localhost:3000/test/repo|test/repo>", false,
		},
		{
			"Push", func() (SlackPayload, error) { return sc.Push(pushTestPayload()) },
			"[<http://localhost:3000/test/repo|test/repo>:<http://localhost:3000/test/repo/src/branch/test|test>] 2 new commits pushed by user1", true,
		},
		{
			"IssueComment", func() (SlackPayload, error) { return sc.IssueComment(issueCommentTestPayload()) },
			"[<http://localhost:3000/test/repo|test/repo>] New comment on issue <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", true,
		},
		{
			"PullRequest", func() (SlackPayload, error) { return sc.PullRequest(pullRequestTestPayload()) },
			"[<http://localhost:3000/test/repo|test/repo>] Pull request opened: <http://localhost:3000/test/repo/pulls/12|#12 Fix bug> by <https://try.gitea.io/user1|user1>", true,
		},
		{
			"PullRequestComment", func() (SlackPayload, error) { return sc.IssueComment(pullRequestCommentTestPayload()) },
			"[<http://localhost:3000/test/repo|test/repo>] New comment on pull request <http://localhost:3000/test/repo/pulls/12|#12 Fix bug> by <https://try.gitea.io/user1|user1>", false,
		},
		{
			"Review", func() (SlackPayload, error) {
				p := pullRequestTestPayload()
				p.Action = api.HookIssueReviewed
				return sc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
			},
			"[<http://localhost:3000/test/repo|test/repo>] Pull request review approved: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by <https://try.gitea.io/user1|user1>", false,
		},
		{
			"Package", func() (SlackPayload, error) { return sc.Package(packageTestPayload()) },
			"Package created: <http://localhost:3000/user1/-/packages/container/GiteaContainer/latest|GiteaContainer:latest> by <https://try.gitea.io/user1|user1>", false,
		},
		{
			"Release", func() (SlackPayload, error) { return sc.Release(pullReleaseTestPayload()) },
			"[<http://localhost:3000/test/repo|test/repo>] Release created: <http://localhost:3000/test/repo/releases/tag/v1.0|v1.0> by <https://try.gitea.io/user1|user1>", false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			pl, err := testCase.call()
			require.NoError(t, err)
			assert.Equal(t, testCase.text, pl.Text)
			if testCase.attachments {
				require.Len(t, pl.Attachments, 1)
				assert.Equal(t, "user1", pl.Attachments[0].AuthorName)
				assert.Equal(t, "http://localhost:3000/user1/avatar", pl.Attachments[0].AuthorIcon)
				assert.Equal(t, "http://localhost:3000/user1", pl.Attachments[0].AuthorLink)
			}
		})
	}

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := sc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue opened: <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.Text)
		require.Len(t, pl.Attachments, 1)
		assert.Equal(t, "user1", pl.Attachments[0].AuthorName)
		assert.Equal(t, "http://localhost:3000/user1/avatar", pl.Attachments[0].AuthorIcon)
		assert.Equal(t, "http://localhost:3000/user1", pl.Attachments[0].AuthorLink)

		p.Action = api.HookIssueClosed
		pl, err = sc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Issue closed: <http://localhost:3000/test/repo/issues/2|#2 crash> by <https://try.gitea.io/user1|user1>", pl.Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := sc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Repository created by <https://try.gitea.io/user1|user1>", pl.Text)

		p.Action = api.HookRepoRenamed
		p.Changes = &api.ChangesPayload{Name: &api.ChangesFromPayload{From: "old-repo"}}
		pl, err = sc.Repository(p)
		require.NoError(t, err)
		assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>] Repository renamed from old-repo by <https://try.gitea.io/user1|user1>", pl.Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()
		for _, testCase := range []struct {
			action api.HookWikiAction
			text   string
		}{
			{api.HookWikiCreated, "[<http://localhost:3000/test/repo|test/repo>] New wiki page '<http://localhost:3000/test/repo/wiki/index|index>' (Wiki change comment) by <https://try.gitea.io/user1|user1>"},
			{api.HookWikiEdited, "[<http://localhost:3000/test/repo|test/repo>] Wiki page '<http://localhost:3000/test/repo/wiki/index|index>' edited (Wiki change comment) by <https://try.gitea.io/user1|user1>"},
			{api.HookWikiDeleted, "[<http://localhost:3000/test/repo|test/repo>] Wiki page '<http://localhost:3000/test/repo/wiki/index|index>' deleted by <https://try.gitea.io/user1|user1>"},
		} {
			p.Action = testCase.action
			pl, err := sc.Wiki(p)
			require.NoError(t, err)
			assert.Equal(t, testCase.text, pl.Text)
		}
	})
}

func TestSlackJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.SLACK,
		URL:        "https://slack.example.com/",
		Meta:       `{}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newSlackRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://slack.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body SlackPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, "[<http://localhost:3000/test/repo|test/repo>:<http://localhost:3000/test/repo/src/branch/test|test>] 2 new commits pushed by user1", body.Text)
}
