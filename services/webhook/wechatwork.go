// Copyright 2021 The Gitea Authors. All rights reserved.
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
	// WechatworkPayload represents
	WechatworkPayload struct {
		Msgtype string `json:"msgtype"`
		Text    struct {
			Content             string   `json:"content"`
			MentionedList       []string `json:"mentioned_list"`
			MentionedMobileList []string `json:"mentioned_mobile_list"`
		} `json:"text"`
		Markdown struct {
			Content string `json:"content"`
		} `json:"markdown"`
	}
)

func newWechatworkMarkdownPayload(title string) WechatworkPayload {
	return WechatworkPayload{
		Msgtype: "markdown",
		Markdown: struct {
			Content string `json:"content"`
		}{
			Content: title,
		},
	}
}

type wechatworkConvertor struct{}

// Create implements PayloadConvertor Create method
func (wc wechatworkConvertor) Create(p *api.CreatePayload) (WechatworkPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return newWechatworkMarkdownPayload(title), nil
}

// Delete implements PayloadConvertor Delete method
func (wc wechatworkConvertor) Delete(p *api.DeletePayload) (WechatworkPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return newWechatworkMarkdownPayload(title), nil
}

// Fork implements PayloadConvertor Fork method
func (wc wechatworkConvertor) Fork(p *api.ForkPayload) (WechatworkPayload, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return newWechatworkMarkdownPayload(title), nil
}

// Push implements PayloadConvertor Push method
func (wc wechatworkConvertor) Push(p *api.PushPayload) (WechatworkPayload, error) {
	var (
		branchName = git.RefName(p.Ref).ShortName()
		commitDesc string
	)

	title := fmt.Sprintf("# %s:%s <font color=\"warning\">  %s  </font>", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = "Author: " + commit.Author.Name
		}

		message := strings.ReplaceAll(commit.Message, "\n\n", "\r\n")
		text += fmt.Sprintf(" > [%s](%s) \r\n ><font color=\"info\">%s</font> \n ><font color=\"warning\">%s</font>", commit.ID[:7], commit.URL,
			message, authorName)

		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}
	return newWechatworkMarkdownPayload(title + "\r\n\r\n" + text), nil
}

// Issue implements PayloadConvertor Issue method
func (wc wechatworkConvertor) Issue(p *api.IssuePayload) (WechatworkPayload, error) {
	text, issueTitle, extraMarkdown, _ := getIssuesPayloadInfo(p, noneLinkFormatter, true)
	var content string
	content += fmt.Sprintf(" ><font color=\"info\">%s</font>\n >%s \n ><font color=\"warning\"> %s</font> \n [%s](%s)", text, extraMarkdown, issueTitle, p.Issue.HTMLURL, p.Issue.HTMLURL)

	return newWechatworkMarkdownPayload(content), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (wc wechatworkConvertor) IssueComment(p *api.IssueCommentPayload) (WechatworkPayload, error) {
	text, issueTitle, _ := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)
	var content string
	content += fmt.Sprintf(" ><font color=\"info\">%s</font>\n >%s \n ><font color=\"warning\">%s</font> \n [%s](%s)", text, p.Comment.Body, issueTitle, p.Comment.HTMLURL, p.Comment.HTMLURL)

	return newWechatworkMarkdownPayload(content), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (wc wechatworkConvertor) PullRequest(p *api.PullRequestPayload) (WechatworkPayload, error) {
	text, issueTitle, extraMarkdown, _ := getPullRequestPayloadInfo(p, noneLinkFormatter, true)
	pr := fmt.Sprintf("> <font color=\"info\"> %s </font> \r\n > <font color=\"comment\">%s </font> \r\n > <font color=\"comment\">%s </font> \r\n",
		text, issueTitle, extraMarkdown)

	return newWechatworkMarkdownPayload(pr), nil
}

// Review implements PayloadConvertor Review method
func (wc wechatworkConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (WechatworkPayload, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return WechatworkPayload{}, err
		}
		title = fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content
	}

	return newWechatworkMarkdownPayload("# " + title + "\r\n\r\n >" + text), nil
}

// Repository implements PayloadConvertor Repository method
func (wc wechatworkConvertor) Repository(p *api.RepositoryPayload) (WechatworkPayload, error) {
	var title string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return newWechatworkMarkdownPayload(title), nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return newWechatworkMarkdownPayload(title), nil
	}

	return WechatworkPayload{}, nil
}

// Wiki implements PayloadConvertor Wiki method
func (wc wechatworkConvertor) Wiki(p *api.WikiPayload) (WechatworkPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, noneLinkFormatter, true)

	return newWechatworkMarkdownPayload(text), nil
}

// Release implements PayloadConvertor Release method
func (wc wechatworkConvertor) Release(p *api.ReleasePayload) (WechatworkPayload, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return newWechatworkMarkdownPayload(text), nil
}

func (wc wechatworkConvertor) Package(p *api.PackagePayload) (WechatworkPayload, error) {
	text, _ := getPackagePayloadInfo(p, noneLinkFormatter, true)

	return newWechatworkMarkdownPayload(text), nil
}

func (wc wechatworkConvertor) Status(p *api.CommitStatusPayload) (WechatworkPayload, error) {
	text, _ := getStatusPayloadInfo(p, noneLinkFormatter, true)

	return newWechatworkMarkdownPayload(text), nil
}

func newWechatworkRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	var pc payloadConvertor[WechatworkPayload] = wechatworkConvertor{}
	return newJSONRequest(pc, w, t, true)
}

func init() {
	RegisterWebhookRequester(webhook_module.WECHATWORK, newWechatworkRequest)
}
