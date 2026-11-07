// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"testing"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMSTeamsPayload(t *testing.T) {
	mc := msteamsConvertor{}

	checkPayload := func(t *testing.T, pl MSTeamsPayload, title, text string, facts map[string]string) {
		assert.Equal(t, title, pl.Title)
		assert.Equal(t, title, pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		if text == "" {
			assert.Empty(t, pl.Sections[0].Text)
		} else {
			assert.Equal(t, text, pl.Sections[0].Text)
		}
		assert.Len(t, pl.Sections[0].Facts, len(facts))
		for _, fact := range pl.Sections[0].Facts {
			if expected, ok := facts[fact.Name]; ok {
				assert.Equal(t, expected, fact.Value)
			} else {
				t.Fail()
			}
		}
	}
	checkAction := func(t *testing.T, pl MSTeamsPayload, target string) {
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, target, pl.PotentialAction[0].Targets[0].URI)
	}
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := mc.Create(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] branch test created", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repo.FullName, p.Repo.HTMLURL),
			"branch:":     "test",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := mc.Delete(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] branch test deleted", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repo.FullName, p.Repo.HTMLURL),
			"branch:":     "test",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := mc.Fork(p)
		require.NoError(t, err)

		checkPayload(t, pl, "test/repo2 is forked to test/repo", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repo.FullName, p.Repo.HTMLURL),
			"Forkee:":     p.Forkee.FullName,
		})
		checkAction(t, pl, "http://localhost:3000/test/repo")
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := mc.Push(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo:test] 2 new commits", "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\n\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", map[string]string{
			"Repository:":   fmt.Sprintf("[%s](%s)", p.Repo.FullName, p.Repo.HTMLURL),
			"Commit count:": "2",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/src/test")
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := mc.Issue(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Issue opened: #2 crash", "issue body", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
			"Issue #:":    "2",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/issues/2")

		p.Action = api.HookIssueClosed
		pl, err = mc.Issue(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Issue closed: #2 crash", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
			"Issue #:":    "2",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/issues/2")
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] New comment on issue #2 crash", "more info needed", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
			"Issue #:":    "2",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/issues/2#issuecomment-4")
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := mc.PullRequest(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Pull request opened: #12 Fix bug", "fixes bug #2", map[string]string{
			"Repository:":   fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
			"Pull request:": fmt.Sprintf("[#%d](%s)", p.PullRequest.Index, p.PullRequest.HTMLURL),
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/pulls/12")
	})

	t.Run("PullRequestReviewRequest", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewRequested
		p.RequestedReviewer = &api.User{
			UserName: "reviewer1",
			FullName: "Reviewer One",
		}

		pl, err := mc.PullRequest(p)
		require.NoError(t, err)

		assert.Len(t, pl.Sections[0].Facts, 3)
		var hasReviewer bool
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL), fact.Value)
			} else if fact.Name == "Pull request:" {
				assert.Equal(t, fmt.Sprintf("[#%d](%s)", p.PullRequest.Index, p.PullRequest.HTMLURL), fact.Value)
			} else if fact.Name == "Requested Reviewer:" {
				assert.Equal(t, "reviewer1 (Reviewer One)", fact.Value)
				hasReviewer = true
			} else {
				t.Fail()
			}
		}
		assert.True(t, hasReviewer)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] New comment on pull request #12 Fix bug", "changes requested", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
			"Issue #:":    "12",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/pulls/12#issuecomment-4")
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := mc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Pull request review approved: #12 Fix bug", "good job", map[string]string{
			"Repository:":   fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
			"Pull request:": fmt.Sprintf("[#%d](%s)", p.PullRequest.Index, p.PullRequest.HTMLURL),
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/pulls/12")
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := mc.Repository(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Repository created", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
		})
		checkAction(t, pl, "http://localhost:3000/test/repo")
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := mc.Package(p)
		require.NoError(t, err)

		checkPayload(t, pl, "Package created: GiteaContainer:latest", "", map[string]string{
			"Package:": p.Package.Name,
		})
		checkAction(t, pl, "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest")
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := mc.Wiki(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] New wiki page 'index' (Wiki change comment)", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
		})

		p.Action = api.HookWikiEdited
		pl, err = mc.Wiki(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Wiki page 'index' edited (Wiki change comment)", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/wiki/index")

		p.Action = api.HookWikiDeleted
		pl, err = mc.Wiki(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Wiki page 'index' deleted", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/wiki/index")
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := mc.Release(p)
		require.NoError(t, err)

		checkPayload(t, pl, "[test/repo] Release created: v1.0", "", map[string]string{
			"Repository:": fmt.Sprintf("[%s](%s)", p.Repository.FullName, p.Repository.HTMLURL),
			"Tag:":        "v1.0",
		})
		checkAction(t, pl, "http://localhost:3000/test/repo/releases/tag/v1.0")
	})
}

func TestMSTeamsJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	hook := &webhook_model.Webhook{
		RepoID:     3,
		IsActive:   true,
		Type:       webhook_module.MSTEAMS,
		URL:        "https://msteams.example.com/",
		Meta:       ``,
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newMSTeamsRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, "https://msteams.example.com/", req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var body MSTeamsPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	assert.NoError(t, err)
	assert.Equal(t, "[test/repo:test] 2 new commits", body.Summary)
}
