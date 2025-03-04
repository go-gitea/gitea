// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

type (
	// MSTeamsFact for Fact Structure
	MSTeamsFact struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	// MSTeamsSection is a MessageCard section
	MSTeamsSection struct {
		ActivityTitle    string        `json:"activityTitle"`
		ActivitySubtitle string        `json:"activitySubtitle"`
		ActivityImage    string        `json:"activityImage"`
		Facts            []MSTeamsFact `json:"facts"`
		Text             string        `json:"text"`
	}

	// MSTeamsAction is an action (creates buttons, links etc)
	MSTeamsAction struct {
		Type    string                `json:"@type"`
		Name    string                `json:"name"`
		Targets []MSTeamsActionTarget `json:"targets,omitempty"`
	}

	// MSTeamsActionTarget is the actual link to follow, etc
	MSTeamsActionTarget struct {
		Os  string `json:"os"`
		URI string `json:"uri"`
	}

	// MSTeamsPayload is the parent object
	MSTeamsPayload struct {
		Type            string           `json:"@type"`
		Context         string           `json:"@context"`
		ThemeColor      string           `json:"themeColor"`
		Title           string           `json:"title"`
		Summary         string           `json:"summary"`
		Sections        []MSTeamsSection `json:"sections"`
		PotentialAction []MSTeamsAction  `json:"potentialAction"`
	}
)

type msteamsConvertor struct{}

// Create implements PayloadConvertor Create method
func (m msteamsConvertor) Create(p *api.CreatePayload) (MSTeamsPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName),
		greenColor,
		&MSTeamsFact{fmt.Sprintf("%s:", p.RefType), refName},
	), nil
}

// Delete implements PayloadConvertor Delete method
func (m msteamsConvertor) Delete(p *api.DeletePayload) (MSTeamsPayload, error) {
	// deleted tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName),
		yellowColor,
		&MSTeamsFact{fmt.Sprintf("%s:", p.RefType), refName},
	), nil
}

// Fork implements PayloadConvertor Fork method
func (m msteamsConvertor) Fork(p *api.ForkPayload) (MSTeamsPayload, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.Repo.HTMLURL,
		greenColor,
		&MSTeamsFact{"Forkee:", p.Forkee.FullName},
	), nil
}

// Push implements PayloadConvertor Push method
func (m msteamsConvertor) Push(p *api.PushPayload) (MSTeamsPayload, error) {
	var (
		branchName = git.RefName(p.Ref).ShortName()
		commitDesc string
	)

	var titleLink string
	if p.TotalCommits == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
	} else {
		commitDesc = fmt.Sprintf("%d new commits", p.TotalCommits)
		titleLink = p.CompareURL
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + util.PathEscapeSegments(branchName)
	}

	title := fmt.Sprintf("[%s:%s] %s", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		text += fmt.Sprintf("[%s](%s) %s - %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n"), commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n\n"
		}
	}

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		text,
		titleLink,
		greenColor,
		&MSTeamsFact{"Commit count:", fmt.Sprintf("%d", p.TotalCommits)},
	), nil
}

// Issue implements PayloadConvertor Issue method
func (m msteamsConvertor) Issue(p *api.IssuePayload) (MSTeamsPayload, error) {
	title, _, extraMarkdown, color := getIssuesPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		extraMarkdown,
		p.Issue.HTMLURL,
		color,
		&MSTeamsFact{"Issue #:", fmt.Sprintf("%d", p.Issue.ID)},
	), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (m msteamsConvertor) IssueComment(p *api.IssueCommentPayload) (MSTeamsPayload, error) {
	title, _, color := getIssueCommentPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		p.Comment.Body,
		p.Comment.HTMLURL,
		color,
		&MSTeamsFact{"Issue #:", fmt.Sprintf("%d", p.Issue.ID)},
	), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (m msteamsConvertor) PullRequest(p *api.PullRequestPayload) (MSTeamsPayload, error) {
	title, _, extraMarkdown, color := getPullRequestPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		extraMarkdown,
		p.PullRequest.HTMLURL,
		color,
		&MSTeamsFact{"Pull request #:", fmt.Sprintf("%d", p.PullRequest.ID)},
	), nil
}

// Review implements PayloadConvertor Review method
func (m msteamsConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (MSTeamsPayload, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return MSTeamsPayload{}, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content

		switch event {
		case webhook_module.HookEventPullRequestReviewApproved:
			color = greenColor
		case webhook_module.HookEventPullRequestReviewRejected:
			color = redColor
		case webhook_module.HookEventPullRequestReviewComment:
			color = greyColor
		default:
			color = yellowColor
		}
	}

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		text,
		p.PullRequest.HTMLURL,
		color,
		&MSTeamsFact{"Pull request #:", fmt.Sprintf("%d", p.PullRequest.ID)},
	), nil
}

// Repository implements PayloadConvertor Repository method
func (m msteamsConvertor) Repository(p *api.RepositoryPayload) (MSTeamsPayload, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		color = greenColor
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		color = yellowColor
	}

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		"",
		url,
		color,
		nil,
	), nil
}

// Wiki implements PayloadConvertor Wiki method
func (m msteamsConvertor) Wiki(p *api.WikiPayload) (MSTeamsPayload, error) {
	title, color, _ := getWikiPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		"",
		p.Repository.HTMLURL+"/wiki/"+url.PathEscape(p.Page),
		color,
		&MSTeamsFact{"Repository:", p.Repository.FullName},
	), nil
}

// Release implements PayloadConvertor Release method
func (m msteamsConvertor) Release(p *api.ReleasePayload) (MSTeamsPayload, error) {
	title, color := getReleasePayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		"",
		p.Release.HTMLURL,
		color,
		&MSTeamsFact{"Tag:", p.Release.TagName},
	), nil
}

func (m msteamsConvertor) Package(p *api.PackagePayload) (MSTeamsPayload, error) {
	title, color := getPackagePayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		"",
		p.Package.HTMLURL,
		color,
		&MSTeamsFact{"Package:", p.Package.Name},
	), nil
}

func (m msteamsConvertor) Status(p *api.CommitStatusPayload) (MSTeamsPayload, error) {
	title, color := getStatusPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.TargetURL,
		color,
		&MSTeamsFact{"CommitStatus:", p.Context},
	), nil
}

func (msteamsConvertor) WorkflowJob(p *api.WorkflowJobPayload) (MSTeamsPayload, error) {
	title, color := getWorkflowJobPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.WorkflowJob.HTMLURL,
		color,
		&MSTeamsFact{"WorkflowJob:", p.WorkflowJob.Name},
	), nil
}

func createMSTeamsPayload(r *api.Repository, s *api.User, title, text, actionTarget string, color int, fact *MSTeamsFact) MSTeamsPayload {
	facts := make([]MSTeamsFact, 0, 2)
	if r != nil {
		facts = append(facts, MSTeamsFact{
			Name:  "Repository:",
			Value: r.FullName,
		})
	}
	if fact != nil {
		facts = append(facts, *fact)
	}

	return MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    s.FullName,
				ActivitySubtitle: s.UserName,
				ActivityImage:    s.AvatarURL,
				Text:             text,
				Facts:            facts,
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: actionTarget,
					},
				},
			},
		},
	}
}

func newMSTeamsRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	var pc payloadConvertor[MSTeamsPayload] = msteamsConvertor{}
	return newJSONRequest(pc, w, t, true)
}

func init() {
	RegisterWebhookRequester(webhook_module.MSTEAMS, newMSTeamsRequest)
}
