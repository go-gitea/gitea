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

type feishuConvertor struct{}

// Create implements PayloadConvertor Create method
func (fc feishuConvertor) Create(p *api.CreatePayload) (FeishuPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	text := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Delete implements PayloadConvertor Delete method
func (fc feishuConvertor) Delete(p *api.DeletePayload) (FeishuPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	text := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Fork implements PayloadConvertor Fork method
func (fc feishuConvertor) Fork(p *api.ForkPayload) (FeishuPayload, error) {
	text := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return newFeishuTextPayload(text), nil
}

// Push implements PayloadConvertor Push method
func (fc feishuConvertor) Push(p *api.PushPayload) (FeishuPayload, error) {
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

// Issue implements PayloadConvertor Issue method
func (fc feishuConvertor) Issue(p *api.IssuePayload) (FeishuPayload, error) {
	title, link, by, operator, result, assignees := getIssuesInfo(p)
	if assignees != "" {
		if p.Action == api.HookIssueAssigned || p.Action == api.HookIssueUnassigned || p.Action == api.HookIssueMilestoned {
			return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, result, assignees, p.Issue.Body)), nil
		}
		return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, assignees, p.Issue.Body)), nil
	}
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.Issue.Body)), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (fc feishuConvertor) IssueComment(p *api.IssueCommentPayload) (FeishuPayload, error) {
	title, link, by, operator := getIssuesCommentInfo(p)
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.Comment.Body)), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (fc feishuConvertor) PullRequest(p *api.PullRequestPayload) (FeishuPayload, error) {
	title, link, by, operator, result, assignees := getPullRequestInfo(p)
	if assignees != "" {
		if p.Action == api.HookIssueAssigned || p.Action == api.HookIssueUnassigned || p.Action == api.HookIssueMilestoned {
			return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, result, assignees, p.PullRequest.Body)), nil
		}
		return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, assignees, p.PullRequest.Body)), nil
	}
	return newFeishuTextPayload(fmt.Sprintf("%s\n%s\n%s\n%s\n\n%s", title, link, by, operator, p.PullRequest.Body)), nil
}

// Review implements PayloadConvertor Review method
func (fc feishuConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (FeishuPayload, error) {
	action, err := parseHookPullRequestEventType(event)
	if err != nil {
		return FeishuPayload{}, err
	}

	title := fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
	text := p.Review.Content

	return newFeishuTextPayload(title + "\r\n\r\n" + text), nil
}

// Repository implements PayloadConvertor Repository method
func (fc feishuConvertor) Repository(p *api.RepositoryPayload) (FeishuPayload, error) {
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

// Wiki implements PayloadConvertor Wiki method
func (fc feishuConvertor) Wiki(p *api.WikiPayload) (FeishuPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

// Release implements PayloadConvertor Release method
func (fc feishuConvertor) Release(p *api.ReleasePayload) (FeishuPayload, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func (fc feishuConvertor) Package(p *api.PackagePayload) (FeishuPayload, error) {
	text, _ := getPackagePayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

func newFeishuRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	var pc payloadConvertor[FeishuPayload] = feishuConvertor{}
	return newJSONRequest(pc, w, t, true)
}
