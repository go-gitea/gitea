// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

type (
	// FeishuPayload represents
	FeishuPayload struct {
		MsgType string `json:"msg_type"` // text / post / image / share_chat / interactive / file /audio / media
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	}
)

func newFeishuTextPayload(text string) FeishuPayload {
	return FeishuPayload{
		MsgType: "text",
		Content: struct {
			Text string `json:"text"`
		}{
			Text: strings.TrimSpace(text),
		},
	}
}

// Create implements PayloadConverter Create method
func (fc feishuConverter) Create(p *api.CreatePayload) (FeishuPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	text := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Delete implements PayloadConverter Delete method
func (fc feishuConverter) Delete(p *api.DeletePayload) (FeishuPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	text := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Fork implements PayloadConverter Fork method
func (fc feishuConverter) Fork(p *api.ForkPayload) (FeishuPayload, error) {
	text := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return newFeishuTextPayload(text), nil
}

// Push implements PayloadConverter Push method
func (fc feishuConverter) Push(p *api.PushPayload) (FeishuPayload, error) {
	var (
		branchName = git.RefName(p.Ref).ShortName()
		commitDesc string
	)

	text := fmt.Sprintf("[%s:%s] %s\r\n", p.Repo.FullName, branchName, commitDesc)
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = " - " + commit.Author.Name
		}
		text += fmt.Sprintf("[%s](%s) %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n")) + authorName
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\r\n"
		}
	}

	return newFeishuTextPayload(text), nil
}

// Issue implements PayloadConverter Issue method
func (fc feishuConverter) Issue(p *api.IssuePayload) (FeishuPayload, error) {
	title, link, by, operator, result, assignees := getIssuesInfo(p)
	if assignees != "" {
		if p.Action == api.HookIssueAssigned || p.Action == api.HookIssueUnassigned || p.Action == api.HookIssueMilestoned {
			return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, result, assignees, p.Issue.Body)), nil
		}
		return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, assignees, p.Issue.Body)), nil
	}
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.Issue.Body)), nil
}

// IssueComment implements PayloadConverter IssueComment method
func (fc feishuConverter) IssueComment(p *api.IssueCommentPayload) (FeishuPayload, error) {
	title, link, by, operator := getIssuesCommentInfo(p)
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.Comment.Body)), nil
}

// PullRequest implements PayloadConverter PullRequest method
func (fc feishuConverter) PullRequest(p *api.PullRequestPayload) (FeishuPayload, error) {
	title, link, by, operator, result, assignees := getPullRequestInfo(p)
	if assignees != "" {
		if p.Action == api.HookIssueAssigned || p.Action == api.HookIssueUnassigned || p.Action == api.HookIssueMilestoned {
			return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, result, assignees, p.PullRequest.Body)), nil
		}
		return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, assignees, p.PullRequest.Body)), nil
	}
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.PullRequest.Body)), nil
}

// Review implements PayloadConverter Review method
func (fc feishuConverter) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (FeishuPayload, error) {
	action, err := parseHookPullRequestEventType(event)
	if err != nil {
		return FeishuPayload{}, err
	}

	title := fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
	text := p.Review.Content

	return newFeishuTextPayload(title + "\r\n\r\n" + text), nil
}

// Repository implements PayloadConverter Repository method
func (fc feishuConverter) Repository(p *api.RepositoryPayload) (FeishuPayload, error) {
	var text string
	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return newFeishuTextPayload(text), nil
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return newFeishuTextPayload(text), nil
	}

	return FeishuPayload{}, nil
}

// Wiki implements PayloadConverter Wiki method
func (fc feishuConverter) Wiki(p *api.WikiPayload) (FeishuPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

// Release implements PayloadConverter Release method
func (fc feishuConverter) Release(p *api.ReleasePayload) (FeishuPayload, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func (fc feishuConverter) Package(p *api.PackagePayload) (FeishuPayload, error) {
	text, _ := getPackagePayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

type feishuConverter struct{}

var _ payloadConverter[FeishuPayload] = feishuConverter{}

func newFeishuRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	return newJSONRequest(feishuConverter{}, w, t, true)
}
