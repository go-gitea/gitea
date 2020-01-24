// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"

	dingtalk "github.com/lunny/dingtalk_webhook"
)

type (
	// DingtalkPayload represents
	DingtalkPayload dingtalk.Payload
)

// SetSecret sets the dingtalk secret
func (p *DingtalkPayload) SetSecret(_ string) {}

// JSONPayload Marshals the DingtalkPayload to json
func (p *DingtalkPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func getDingtalkCreatePayload(p *api.CreatePayload) (*DingtalkPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        title,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: fmt.Sprintf("view ref %s", refName),
			SingleURL:   p.Repo.HTMLURL + "/src/" + refName,
		},
	}, nil
}

func getDingtalkDeletePayload(p *api.DeletePayload) (*DingtalkPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        title,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: fmt.Sprintf("view ref %s", refName),
			SingleURL:   p.Repo.HTMLURL + "/src/" + refName,
		},
	}, nil
}

func getDingtalkForkPayload(p *api.ForkPayload) (*DingtalkPayload, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        title,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: fmt.Sprintf("view forked repo %s", p.Repo.FullName),
			SingleURL:   p.Repo.HTMLURL,
		},
	}, nil
}

func getDingtalkPushPayload(p *api.PushPayload) (*DingtalkPayload, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

	var titleLink, linkText string
	if len(p.Commits) == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
		linkText = fmt.Sprintf("view commit %s", p.Commits[0].ID[:7])
	} else {
		commitDesc = fmt.Sprintf("%d new commits", len(p.Commits))
		titleLink = p.CompareURL
		linkText = fmt.Sprintf("view commit %s...%s", p.Commits[0].ID[:7], p.Commits[len(p.Commits)-1].ID[:7])
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + branchName
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
			text += "\n"
		}
	}

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        text,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: linkText,
			SingleURL:   titleLink,
		},
	}, nil
}

func getDingtalkIssuesPayload(p *api.IssuePayload) (*DingtalkPayload, error) {
	text, issueTitle, attachmentText, _ := getIssuesPayloadInfo(p, noneLinkFormatter, true)

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text: text + "\r\n\r\n" + attachmentText,
			//Markdown:    "# " + title + "\n" + text,
			Title:       issueTitle,
			HideAvatar:  "0",
			SingleTitle: "view issue",
			SingleURL:   p.Issue.HTMLURL,
		},
	}, nil
}

func getDingtalkIssueCommentPayload(p *api.IssueCommentPayload) (*DingtalkPayload, error) {
	text, issueTitle, _ := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        text + "\r\n\r\n" + p.Comment.Body,
			Title:       issueTitle,
			HideAvatar:  "0",
			SingleTitle: "view issue comment",
			SingleURL:   p.Comment.HTMLURL,
		},
	}, nil
}

func getDingtalkPullRequestPayload(p *api.PullRequestPayload) (*DingtalkPayload, error) {
	text, issueTitle, attachmentText, _ := getPullRequestPayloadInfo(p, noneLinkFormatter, true)

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text: text + "\r\n\r\n" + attachmentText,
			//Markdown:    "# " + title + "\n" + text,
			Title:       issueTitle,
			HideAvatar:  "0",
			SingleTitle: "view pull request",
			SingleURL:   p.PullRequest.HTMLURL,
		},
	}, nil
}

func getDingtalkPullRequestApprovalPayload(p *api.PullRequestPayload, event models.HookEventType) (*DingtalkPayload, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content

	}

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        title + "\r\n\r\n" + text,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: "view pull request",
			SingleURL:   p.PullRequest.HTMLURL,
		},
	}, nil
}

func getDingtalkRepositoryPayload(p *api.RepositoryPayload) (*DingtalkPayload, error) {
	var title, url string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		return &DingtalkPayload{
			MsgType: "actionCard",
			ActionCard: dingtalk.ActionCard{
				Text:        title,
				Title:       title,
				HideAvatar:  "0",
				SingleTitle: "view repository",
				SingleURL:   url,
			},
		}, nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return &DingtalkPayload{
			MsgType: "text",
			Text: struct {
				Content string `json:"content"`
			}{
				Content: title,
			},
		}, nil
	}

	return nil, nil
}

func getDingtalkReleasePayload(p *api.ReleasePayload) (*DingtalkPayload, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        text,
			Title:       text,
			HideAvatar:  "0",
			SingleTitle: "view release",
			SingleURL:   p.Release.URL,
		},
	}, nil
}

// GetDingtalkPayload converts a ding talk webhook into a DingtalkPayload
func GetDingtalkPayload(p api.Payloader, event models.HookEventType, meta string) (*DingtalkPayload, error) {
	s := new(DingtalkPayload)

	switch event {
	case models.HookEventCreate:
		return getDingtalkCreatePayload(p.(*api.CreatePayload))
	case models.HookEventDelete:
		return getDingtalkDeletePayload(p.(*api.DeletePayload))
	case models.HookEventFork:
		return getDingtalkForkPayload(p.(*api.ForkPayload))
	case models.HookEventIssues:
		return getDingtalkIssuesPayload(p.(*api.IssuePayload))
	case models.HookEventIssueComment:
		return getDingtalkIssueCommentPayload(p.(*api.IssueCommentPayload))
	case models.HookEventPush:
		return getDingtalkPushPayload(p.(*api.PushPayload))
	case models.HookEventPullRequest:
		return getDingtalkPullRequestPayload(p.(*api.PullRequestPayload))
	case models.HookEventPullRequestApproved, models.HookEventPullRequestRejected, models.HookEventPullRequestComment:
		return getDingtalkPullRequestApprovalPayload(p.(*api.PullRequestPayload), event)
	case models.HookEventRepository:
		return getDingtalkRepositoryPayload(p.(*api.RepositoryPayload))
	case models.HookEventRelease:
		return getDingtalkReleasePayload(p.(*api.ReleasePayload))
	}

	return s, nil
}
