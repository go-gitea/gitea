// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"testing"

	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeishuPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, `[test/repo] branch test created`, pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, `[test/repo] branch test deleted`, pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, `test/repo2 is forked to test/repo`, pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo:test] \r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(FeishuPayload)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[Issue-test/repo #2]: opened\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.(*FeishuPayload).Content.Text)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[Issue-test/repo #2]: closed\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\nAssignees: user1\n\nissue body", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(FeishuPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[Comment-test/repo #2]: created\ncrash\nhttp://localhost:3000/test/repo/issues/2\nIssue by user1\nOperator: user1\n\nmore info needed", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(FeishuPayload)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[PullRequest-test/repo #12]: opened\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\nAssignees: user1\n\nfixes bug #2", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(FeishuPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[Comment-test/repo #12]: created\nFix bug\nhttp://localhost:3000/test/repo/pulls/12\nPullRequest by user1\nOperator: user1\n\nchanges requested", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(FeishuPayload)
		pl, err := d.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Repository created", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Package(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "Package created: GiteaContainer:latest by user1", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		d := new(FeishuPayload)
		p.Action = api.HookWikiCreated
		pl, err := d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] New wiki page 'index' (Wiki change comment) by user1", pl.(*FeishuPayload).Content.Text)

		p.Action = api.HookWikiEdited
		pl, err = d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Wiki page 'index' edited (Wiki change comment) by user1", pl.(*FeishuPayload).Content.Text)

		p.Action = api.HookWikiDeleted
		pl, err = d.Wiki(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Wiki page 'index' deleted by user1", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.(*FeishuPayload).Content.Text)
	})
}

func TestFeishuJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(FeishuPayload).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &FeishuPayload{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}
