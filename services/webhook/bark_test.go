// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBarkPayload(t *testing.T) {
	bc := barkConvertor{}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := bc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] branch test created", pl.Title)
		assert.Equal(t, "user1 created branch test", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := bc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] branch test deleted", pl.Title)
		assert.Equal(t, "user1 deleted branch test", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := bc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo2] Repository forked", pl.Title)
		assert.Equal(t, "user1 forked test/repo2 to test/repo", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.URL)
		assert.Equal(t, "test/repo2", pl.Group)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := bc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo:test] 2 new commit(s)", pl.Title)
		assert.Contains(t, pl.Body, "user1 pushed to test")
		assert.Contains(t, pl.Body, "2020558: commit message - user1")
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := bc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue #2: opened", pl.Title)
		assert.Equal(t, "user1 opened issue #2: crash", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)

		p.Action = api.HookIssueClosed
		pl, err = bc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue #2: closed", pl.Title)
		assert.Equal(t, "user1 closed issue #2: crash", pl.Body)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := bc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on #2", pl.Title)
		assert.Contains(t, pl.Body, "user1 commented on issue #2: crash")
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := bc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] PR #12: opened", pl.Title)
		assert.Equal(t, "user1 opened pull request #12: Fix bug", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := bc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on #12", pl.Title)
		assert.Contains(t, pl.Body, "user1 commented")
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := bc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] PR #12 review approved", pl.Title)
		assert.Equal(t, "PR #12: Fix bug", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := bc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Repository created", pl.Title)
		assert.Equal(t, "user1 created repository", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := bc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Package published", pl.Title)
		assert.Contains(t, pl.Body, "user1 published package")
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := bc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki created", pl.Title)
		assert.Equal(t, "user1 created wiki page: index", pl.Body)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := bc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Release published", pl.Title)
		assert.Contains(t, pl.Body, "user1 published release v1.0")
		assert.Equal(t, "http://localhost:3000/test/repo/releases/tag/v1.0", pl.URL)
		assert.Equal(t, "test/repo", pl.Group)
	})
}

func TestBarkJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.BARK,
		URL:        "https://api.day.app/devicekey/",
		Meta:       `{}`,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newBarkRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://api.day.app/devicekey/", req.URL.String())
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}
