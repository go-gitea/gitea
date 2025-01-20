// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscordPayload(t *testing.T) {
	dc := discordConvertor{}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := dc.Create(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] branch test created", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := dc.Delete(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] branch test deleted", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := dc.Fork(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "test/repo2 is forked to test/repo", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Embeds[0].Title)
		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PushWithMultilineCommitMessage", func(t *testing.T) {
		p := pushTestMultilineCommitMessagePayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Embeds[0].Title)
		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) chore: This is a commit summary - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) chore: This is a commit summary - user1", pl.Embeds[0].Description)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PushWithLongCommitSummary", func(t *testing.T) {
		p := pushTestPayloadWithCommitMessage("This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好 ⚠️⚠️️\n\nThis is the message body")

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Embeds[0].Title)
		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好... - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好... - user1", pl.Embeds[0].Description)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := dc.Issue(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Issue opened: #2 crash", pl.Embeds[0].Title)
		assert.Equal(t, "issue body", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)

		p.Action = api.HookIssueClosed
		pl, err = dc.Issue(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Issue closed: #2 crash", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] New comment on issue #2 crash", pl.Embeds[0].Title)
		assert.Equal(t, "more info needed", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := dc.PullRequest(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug", pl.Embeds[0].Title)
		assert.Equal(t, "fixes bug #2", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug", pl.Embeds[0].Title)
		assert.Equal(t, "changes requested", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := dc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Pull request review approved: #12 Fix bug", pl.Embeds[0].Title)
		assert.Equal(t, "good job", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := dc.Repository(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Repository created", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := dc.Package(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "Package created: GiteaContainer:latest", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := dc.Wiki(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] New wiki page 'index' (Wiki change comment)", pl.Embeds[0].Title)
		assert.Equal(t, "Wiki change comment", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)

		p.Action = api.HookWikiEdited
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Wiki page 'index' edited (Wiki change comment)", pl.Embeds[0].Title)
		assert.Equal(t, "Wiki change comment", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)

		p.Action = api.HookWikiDeleted
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Wiki page 'index' deleted", pl.Embeds[0].Title)
		assert.Empty(t, pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := dc.Release(p)
		require.NoError(t, err)

		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, "[test/repo] Release created: v1.0", pl.Embeds[0].Title)
		assert.Equal(t, "Note of first stable release", pl.Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/releases/tag/v1.0", pl.Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	})
}

func TestDiscordJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.DISCORD,
		URL:        "https://discord.example.com/",
		Meta:       `{}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newDiscordRequest(context.Background(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://discord.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body DiscordPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", body.Embeds[0].Description)
}
