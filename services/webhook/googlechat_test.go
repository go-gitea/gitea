// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoogleChatPayload(t *testing.T) {
	iconURL := "gitea-notification-icon"
	gc := googleChatConvertor{Name: "test", IconURL: iconURL}

	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()
		pl, err := gc.Create(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		repoLink := googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
		refLink := googleChatLinkToRef(p.Repo.HTMLURL, p.Ref)
		exp := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, googleChatUserLink(p.Sender))
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, exp, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()
		pl, err := gc.Delete(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		refName := git.RefName(p.Ref).ShortName()
		repoLink := googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
		exp := fmt.Sprintf("[%s:%s] %s deleted by %s", repoLink, googleChatTextFormatter(refName), p.RefType, googleChatUserLink(p.Sender))
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, exp, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()
		pl, err := gc.Fork(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		baseLink := googleChatLinkFormatter(p.Forkee.HTMLURL, p.Forkee.FullName)
		forkLink := googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
		exp := fmt.Sprintf("%s is forked to %s", baseLink, forkLink)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, exp, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		pl, err := gc.Push(p)
		require.NoError(t, err)

		assert.Empty(t, pl.Text)
		require.Len(t, pl.CardsV2, 1)
		assert.Equal(t, "gitea-notification", pl.CardsV2[0].CardID)
		assert.Equal(t, "test", pl.CardsV2[0].Card.Header.Title)
		assert.Equal(t, "Gitea Webhook", pl.CardsV2[0].Card.Header.Subtitle)
		assert.Equal(t, iconURL, pl.CardsV2[0].Card.Header.ImageURL)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		assert.Equal(t, fmt.Sprintf("[%s:%s] 2 new commits pushed by %s",
			googleChatLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName),
			googleChatLinkToRef(p.Repo.HTMLURL, p.Ref),
			googleChatUserLink(p.Pusher),
		), widgets[0].TextParagraph.Text)
		commit := p.Commits[0]
		commitLink := googleChatLinkFormatter(commit.URL, commit.ID[:7])
		commitMessage := googleChatTextFormatter(strings.TrimRight(strings.SplitN(commit.Message, "\n", 2)[0], "\r"))
		commitText := fmt.Sprintf("%s: %s - %s", commitLink, commitMessage, googleChatTextFormatter(commit.Author.Name))
		assert.Equal(t, fmt.Sprintf("%s\n%s", commitText, commitText), widgets[1].TextParagraph.Text)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()
		p.Action = api.HookIssueOpened
		pl, err := gc.Issue(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _, extraMarkdown, _ := getIssuesPayloadInfo(p, googleChatLinkFormatter, true)
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		assert.Equal(t, text, widgets[0].TextParagraph.Text)
		assert.Equal(t, googleChatTextFormatter(extraMarkdown), widgets[1].TextParagraph.Text)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()
		pl, err := gc.IssueComment(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _, _ := getIssueCommentPayloadInfo(p, googleChatLinkFormatter, true)
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		assert.Equal(t, text, widgets[0].TextParagraph.Text)
		assert.Equal(t, googleChatTextFormatter(p.Comment.Body), widgets[1].TextParagraph.Text)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()
		pl, err := gc.IssueComment(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _, _ := getIssueCommentPayloadInfo(p, googleChatLinkFormatter, true)
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		assert.Equal(t, text, widgets[0].TextParagraph.Text)
		assert.Equal(t, googleChatTextFormatter(p.Comment.Body), widgets[1].TextParagraph.Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		pl, err := gc.PullRequest(p)
		require.NoError(t, err)

		require.Len(t, pl.CardsV2, 1)
		assert.Equal(t, "test", pl.CardsV2[0].Card.Header.Title)
		assert.Equal(t, "Gitea Webhook", pl.CardsV2[0].Card.Header.Subtitle)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		text, _, extraMarkdown, _ := getPullRequestPayloadInfo(p, googleChatLinkFormatter, true)
		assert.Equal(t, text, widgets[0].TextParagraph.Text)
		assert.Equal(t, googleChatTextFormatter(extraMarkdown), widgets[1].TextParagraph.Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed
		pl, err := gc.Review(p, webhook_module.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		repoLink := googleChatLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
		titleLink := googleChatLinkFormatter(fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index), fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title))
		exp := fmt.Sprintf("[%s] Pull request review %s: %s by %s", repoLink, "approved", titleLink, googleChatUserLink(p.Sender))
		widgets := pl.CardsV2[0].Card.Sections[0].Widgets
		require.Len(t, widgets, 2)
		assert.Equal(t, exp, widgets[0].TextParagraph.Text)
		assert.Equal(t, googleChatTextFormatter(p.Review.Content), widgets[1].TextParagraph.Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()
		pl, err := gc.Repository(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		repoLink := googleChatLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
		exp := fmt.Sprintf("[%s] Repository created by %s", repoLink, googleChatUserLink(p.Sender))
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, exp, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("Wiki", func(t *testing.T) {
		p := wikiTestPayload()
		p.Action = api.HookWikiCreated
		pl, err := gc.Wiki(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _, _ := getWikiPayloadInfo(p, googleChatLinkFormatter, true)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)

		p.Action = api.HookWikiEdited
		pl, err = gc.Wiki(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		text, _, _ = getWikiPayloadInfo(p, googleChatLinkFormatter, true)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)

		p.Action = api.HookWikiDeleted
		pl, err = gc.Wiki(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		text, _, _ = getWikiPayloadInfo(p, googleChatLinkFormatter, true)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()
		pl, err := gc.Release(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _ := getReleasePayloadInfo(p, googleChatLinkFormatter, true)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("Package", func(t *testing.T) {
		p := packageTestPayload()
		pl, err := gc.Package(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _ := getPackagePayloadInfo(p, googleChatLinkFormatter, true)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("Status", func(t *testing.T) {
		p := &api.CommitStatusPayload{
			Context:     "ci/build",
			Description: "Build passed",
			SHA:         "2020558fe2e34debb818a514715839cabd25e778",
			TargetURL:   "http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778/status",
			Sender: &api.User{
				UserName:  "user1",
				AvatarURL: "http://localhost:3000/user1/avatar",
			},
		}
		pl, err := gc.Status(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _ := getStatusPayloadInfo(p, googleChatLinkFormatter, true)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("WorkflowRun", func(t *testing.T) {
		p := &api.WorkflowRunPayload{
			Action: "completed",
			Sender: &api.User{
				UserName:  "user1",
				AvatarURL: "http://localhost:3000/user1/avatar",
			},
			WorkflowRun: &api.ActionWorkflowRun{
				ID:           99,
				HTMLURL:      "http://localhost:3000/test/repo/actions/runs/99",
				DisplayTitle: "Build",
				HeadSha:      "2020558fe2e34debb818a514715839cabd25e778",
				Conclusion:   "success",
			},
		}
		pl, err := gc.WorkflowRun(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _ := getWorkflowRunPayloadInfo(p, googleChatLinkFormatter, true)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})

	t.Run("WorkflowJob", func(t *testing.T) {
		p := &api.WorkflowJobPayload{
			Action: "completed",
			Sender: &api.User{
				UserName:  "user1",
				AvatarURL: "http://localhost:3000/user1/avatar",
			},
			WorkflowJob: &api.ActionWorkflowJob{
				ID:         7,
				HTMLURL:    "http://localhost:3000/test/repo/actions/runs/99/jobs/7",
				RunID:      99,
				Name:       "lint",
				HeadSha:    "2020558fe2e34debb818a514715839cabd25e778",
				Conclusion: "failure",
			},
		}
		pl, err := gc.WorkflowJob(p)
		require.NoError(t, err)
		require.Len(t, pl.CardsV2, 1)
		require.Len(t, pl.CardsV2[0].Card.Sections, 1)
		text, _ := getWorkflowJobPayloadInfo(p, googleChatLinkFormatter, true)
		require.Len(t, pl.CardsV2[0].Card.Sections[0].Widgets, 1)
		assert.Equal(t, text, pl.CardsV2[0].Card.Sections[0].Widgets[0].TextParagraph.Text)
	})
}

func TestGoogleChatJSONPayload(t *testing.T) {
	p := pushTestPayload()
	data, err := p.JSONPayload()
	require.NoError(t, err)

	iconURL := "gitea-notification-icon"
	hookURL := (&url.URL{
		Scheme: "https",
		Host:   "chat.googleapis.com",
		Path:   "/v1/spaces/example/messages",
		RawQuery: url.Values{
			"key":   []string{"key"},
			"token": []string{"token"},
		}.Encode(),
	}).String()
	hook := &webhook_model.Webhook{
		RepoID:     3,
		Name:       "test",
		IsActive:   true,
		Type:       webhook_module.GOOGLECHAT,
		URL:        hookURL,
		Meta:       fmt.Sprintf(`{"icon_url":%q}`, iconURL),
		HTTPMethod: "POST",
	}
	task := &webhook_model.HookTask{
		HookID:         hook.ID,
		EventType:      webhook_module.HookEventPush,
		PayloadContent: string(data),
		PayloadVersion: 2,
	}

	req, reqBody, err := newGoogleChatRequest(t.Context(), hook, task)
	require.NotNil(t, req)
	require.NotNil(t, reqBody)
	require.NoError(t, err)

	assert.Equal(t, "POST", req.Method)
	assert.Equal(t, hook.URL, req.URL.String())
	assert.Equal(t, "sha256=", req.Header.Get("X-Hub-Signature-256"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	var raw map[string]any
	require.NoError(t, json.Unmarshal(reqBody, &raw))
	assert.NotContains(t, raw, "text")

	var body GoogleChatPayload
	err = json.NewDecoder(req.Body).Decode(&body)
	require.NoError(t, err)
	assert.Empty(t, body.Text)
	require.Len(t, body.CardsV2, 1)
	assert.Equal(t, iconURL, body.CardsV2[0].Card.Header.ImageURL)
	assert.Equal(t, "test", body.CardsV2[0].Card.Header.Title)
	assert.Equal(t, "Gitea Webhook", body.CardsV2[0].Card.Header.Subtitle)
	require.Len(t, body.CardsV2[0].Card.Sections[0].Widgets, 2)
}
