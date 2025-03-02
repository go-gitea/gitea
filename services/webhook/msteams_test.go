// Copyright 2021 The Gitea Authors. All rights reserved.
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

func TestMSTeamsPayload(t *testing.T) {
	mc := msteamsConvertor{}
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		pl, err := mc.Create(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] branch test created", pl.Title)
		assert.Equal(t, "[test/repo] branch test created", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "branch:" {
				assert.Equal(t, "test", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		pl, err := mc.Delete(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] branch test deleted", pl.Title)
		assert.Equal(t, "[test/repo] branch test deleted", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "branch:" {
				assert.Equal(t, "test", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		pl, err := mc.Fork(p)
		require.NoError(t, err)

		assert.Equal(t, "test/repo2 is forked to test/repo", pl.Title)
		assert.Equal(t, "test/repo2 is forked to test/repo", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "Forkee:" {
				assert.Equal(t, p.Forkee.FullName, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := mc.Push(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Title)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\n\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "Commit count:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		p.Action = api.HookIssueOpened
		pl, err := mc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue opened: #2 crash", pl.Title)
		assert.Equal(t, "[test/repo] Issue opened: #2 crash", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "issue body", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.PotentialAction[0].Targets[0].URI)

		p.Action = api.HookIssueClosed
		pl, err = mc.Issue(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Issue closed: #2 crash", pl.Title)
		assert.Equal(t, "[test/repo] Issue closed: #2 crash", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on issue #2 crash", pl.Title)
		assert.Equal(t, "[test/repo] New comment on issue #2 crash", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "more info needed", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := mc.PullRequest(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug", pl.Title)
		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "fixes bug #2", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Pull request #:" {
				assert.Equal(t, "12", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		pl, err := mc.IssueComment(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug", pl.Title)
		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "changes requested", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "12", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		pl, err := mc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Pull request review approved: #12 Fix bug", pl.Title)
		assert.Equal(t, "[test/repo] Pull request review approved: #12 Fix bug", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "good job", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Pull request #:" {
				assert.Equal(t, "12", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		pl, err := mc.Repository(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Repository created", pl.Title)
		assert.Equal(t, "[test/repo] Repository created", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 1)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()

		pl, err := mc.Package(p)
		require.NoError(t, err)

		assert.Equal(t, "Package created: GiteaContainer:latest", pl.Title)
		assert.Equal(t, "Package created: GiteaContainer:latest", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 1)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Package:" {
				assert.Equal(t, p.Package.Name, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/user1/-/packages/container/GiteaContainer/latest", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()

		p.Action = api.HookWikiCreated
		pl, err := mc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] New wiki page 'index' (Wiki change comment)", pl.Title)
		assert.Equal(t, "[test/repo] New wiki page 'index' (Wiki change comment)", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.PotentialAction[0].Targets[0].URI)

		p.Action = api.HookWikiEdited
		pl, err = mc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page 'index' edited (Wiki change comment)", pl.Title)
		assert.Equal(t, "[test/repo] Wiki page 'index' edited (Wiki change comment)", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Equal(t, "", pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.PotentialAction[0].Targets[0].URI)

		p.Action = api.HookWikiDeleted
		pl, err = mc.Wiki(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Wiki page 'index' deleted", pl.Title)
		assert.Equal(t, "[test/repo] Wiki page 'index' deleted", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/wiki/index", pl.PotentialAction[0].Targets[0].URI)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		pl, err := mc.Release(p)
		require.NoError(t, err)

		assert.Equal(t, "[test/repo] Release created: v1.0", pl.Title)
		assert.Equal(t, "[test/repo] Release created: v1.0", pl.Summary)
		assert.Len(t, pl.Sections, 1)
		assert.Equal(t, "user1", pl.Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.Sections[0].Text)
		assert.Len(t, pl.Sections[0].Facts, 2)
		for _, fact := range pl.Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Tag:" {
				assert.Equal(t, "v1.0", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.PotentialAction, 1)
		assert.Len(t, pl.PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/releases/tag/v1.0", pl.PotentialAction[0].Targets[0].URI)
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
