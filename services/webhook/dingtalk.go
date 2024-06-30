// Copyright 2017 The Gitea Authors. All rights reserved.
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

	dingtalk "gitea.com/lunny/dingtalk_webhook"
)

type (
	// DingtalkPayload represents
	DingtalkPayload dingtalk.Payload
)

// Create implements PayloadConvertor Create method
func (dc dingtalkConvertor) Create(p *api.CreatePayload) (DingtalkPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return createDingtalkPayload(title, title, fmt.Sprintf("view ref %s", refName), p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName)), nil
}

// Delete implements PayloadConvertor Delete method
func (dc dingtalkConvertor) Delete(p *api.DeletePayload) (DingtalkPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return createDingtalkPayload(title, title, fmt.Sprintf("view ref %s", refName), p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName)), nil
}

// Fork implements PayloadConvertor Fork method
func (dc dingtalkConvertor) Fork(p *api.ForkPayload) (DingtalkPayload, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return createDingtalkPayload(title, title, fmt.Sprintf("view forked repo %s", p.Repo.FullName), p.Repo.HTMLURL), nil
}

// Push implements PayloadConvertor Push method
func (dc dingtalkConvertor) Push(p *api.PushPayload) (DingtalkPayload, error) {
	var (
		branchName = git.RefName(p.Ref).ShortName()
		commitDesc string
	)

	var titleLink, linkText string
	if p.TotalCommits == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
		linkText = "view commit"
	} else {
		commitDesc = fmt.Sprintf("%d new commits", p.TotalCommits)
		titleLink = p.CompareURL
		linkText = "view commits"
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + util.PathEscapeSegments(branchName)
	}

	title := fmt.Sprintf("[%s:%s] %s", p.Repo.FullName, branchName, commitDesc)

	var text string
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

	return createDingtalkPayload(title, text, linkText, titleLink), nil
}

// Issue implements PayloadConvertor Issue method
func (dc dingtalkConvertor) Issue(p *api.IssuePayload) (DingtalkPayload, error) {
	text, issueTitle, attachmentText, _ := getIssuesPayloadInfo(p, noneLinkFormatter, true)

	return createDingtalkPayload(issueTitle, text+"\r\n\r\n"+attachmentText, "view issue", p.Issue.HTMLURL), nil
}

// Wiki implements PayloadConvertor Wiki method
func (dc dingtalkConvertor) Wiki(p *api.WikiPayload) (DingtalkPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, noneLinkFormatter, true)
	url := p.Repository.HTMLURL + "/wiki/" + url.PathEscape(p.Page)

	return createDingtalkPayload(text, text, "view wiki", url), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (dc dingtalkConvertor) IssueComment(p *api.IssueCommentPayload) (DingtalkPayload, error) {
	text, issueTitle, _ := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)

	return createDingtalkPayload(issueTitle, text+"\r\n\r\n"+p.Comment.Body, "view issue comment", p.Comment.HTMLURL), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (dc dingtalkConvertor) PullRequest(p *api.PullRequestPayload) (DingtalkPayload, error) {
	text, issueTitle, attachmentText, _ := getPullRequestPayloadInfo(p, noneLinkFormatter, true)

	return createDingtalkPayload(issueTitle, text+"\r\n\r\n"+attachmentText, "view pull request", p.PullRequest.HTMLURL), nil
}

// Review implements PayloadConvertor Review method
func (dc dingtalkConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (DingtalkPayload, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return DingtalkPayload{}, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content
	}

	return createDingtalkPayload(title, title+"\r\n\r\n"+text, "view pull request", p.PullRequest.HTMLURL), nil
}

// Repository implements PayloadConvertor Repository method
func (dc dingtalkConvertor) Repository(p *api.RepositoryPayload) (DingtalkPayload, error) {
	switch p.Action {
	case api.HookRepoCreated:
		title := fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return createDingtalkPayload(title, title, "view repository", p.Repository.HTMLURL), nil
	case api.HookRepoDeleted:
		title := fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return DingtalkPayload{
			MsgType: "text",
			Text: struct {
				Content string `json:"content"`
			}{
				Content: title,
			},
		}, nil
	}

	return DingtalkPayload{}, nil
}

// Release implements PayloadConvertor Release method
func (dc dingtalkConvertor) Release(p *api.ReleasePayload) (DingtalkPayload, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return createDingtalkPayload(text, text, "view release", p.Release.HTMLURL), nil
}

func (dc dingtalkConvertor) Package(p *api.PackagePayload) (DingtalkPayload, error) {
	text, _ := getPackagePayloadInfo(p, noneLinkFormatter, true)

	return createDingtalkPayload(text, text, "view package", p.Package.HTMLURL), nil
}

func createDingtalkPayload(title, text, singleTitle, singleURL string) DingtalkPayload {
	return DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        strings.TrimSpace(text),
			Title:       strings.TrimSpace(title),
			HideAvatar:  "0",
			SingleTitle: singleTitle,

			// https://developers.dingtalk.com/document/app/message-link-description
			// to open the link in browser, we should use this URL, otherwise the page is displayed inside DingTalk client, very difficult to visit non-public URLs.
			SingleURL: "dingtalk://dingtalkclient/page/link?pc_slide=false&url=" + url.QueryEscape(singleURL),
		},
	}
}

type dingtalkConvertor struct{}

var _ payloadConvertor[DingtalkPayload] = dingtalkConvertor{}

func newDingtalkRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	return newJSONRequest(dingtalkConvertor{}, w, t, true)
}
