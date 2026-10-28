// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscordPayload(t *testing.T) {
	dc := discordConvertor{}
	assertPayload := func(t *testing.T, pl DiscordPayload, sender *api.User, title, description, link string) {
		assert.Len(t, pl.Embeds, 1)
		assert.Equal(t, title, pl.Embeds[0].Title)
		if description == "" {
			assert.Empty(t, pl.Embeds[0].Description)
		} else {
			assert.Equal(t, description, pl.Embeds[0].Description)
		}
		if link != "" {
			assert.Equal(t, link, pl.Embeds[0].URL)
		}
		assert.Equal(t, sender.UserName, pl.Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+sender.UserName, pl.Embeds[0].Author.URL)
		assert.Equal(t, sender.AvatarURL, pl.Embeds[0].Author.IconURL)
	}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := dc.Create(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] branch test created", "", "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := dc.Delete(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] branch test deleted", "", "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := dc.Fork(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "test/repo2 is forked to test/repo", "", "http://localhost:3000/test/repo")
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo:test] 2 new commits", "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", "http://localhost:3000/test/repo/src/test")
	})

	t.Run("PushWithMultilineCommitMessage", func(t *testing.T) {
		p := pushTestMultilineCommitMessagePayload()

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo:test] 2 new commits", "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) chore: This is a commit summary - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) chore: This is a commit summary - user1", "")
	})

	t.Run("PushWithLongCommitSummary", func(t *testing.T) {
		p := pushTestPayloadWithCommitMessage("This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好 ⚠️⚠️️\n\nThis is the message body")

		pl, err := dc.Push(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo:test] 2 new commits", "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好... - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) This is a commit summary ⚠️⚠️⚠️⚠️ containing 你好... - user1", "")
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := dc.Issue(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Issue opened: #2 crash", "issue body", "http://localhost:3000/test/repo/issues/2")

		p.Action = api.HookIssueClosed
		pl, err = dc.Issue(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Issue closed: #2 crash", "", "http://localhost:3000/test/repo/issues/2")
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] New comment on issue #2 crash", "more info needed", "http://localhost:3000/test/repo/issues/2#issuecomment-4")
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := dc.PullRequest(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Pull request opened: #12 Fix bug", "fixes bug #2", "http://localhost:3000/test/repo/pulls/12")
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := dc.IssueComment(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] New comment on pull request #12 Fix bug", "changes requested", "http://localhost:3000/test/repo/pulls/12#issuecomment-4")
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := dc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Pull request review approved: #12 Fix bug", "good job", "http://localhost:3000/test/repo/pulls/12")
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := dc.Repository(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Repository created", "", "http://localhost:3000/test/repo")
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := dc.Package(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "Package created: GiteaContainer:latest", "", "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest")
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := dc.Wiki(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] New wiki page 'index' (Wiki change comment)", "Wiki change comment", "http://localhost:3000/test/repo/wiki/index")

		p.Action = api.HookWikiEdited
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Wiki page 'index' edited (Wiki change comment)", "Wiki change comment", "http://localhost:3000/test/repo/wiki/index")

		p.Action = api.HookWikiDeleted
		pl, err = dc.Wiki(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Wiki page 'index' deleted", "", "http://localhost:3000/test/repo/wiki/index")
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := dc.Release(p)
		require.NoError(t, err)

		assertPayload(t, pl, p.Sender, "[test/repo] Release created: v1.0", "Note of first stable release", "http://localhost:3000/test/repo/releases/tag/v1.0")
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

	req, reqBody, err := newDiscordRequest(t.Context(), hook, task)
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
