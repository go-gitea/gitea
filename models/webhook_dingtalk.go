// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"fmt"
	"strings"

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
	var text, title string
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] Issue opened: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueClosed:
		title = fmt.Sprintf("[%s] Issue closed: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] Issue re-opened: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] Issue edited: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueAssigned:
		title = fmt.Sprintf("[%s] Issue assigned to %s: #%d %s", p.Repository.FullName,
			p.Issue.Assignee.UserName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] Issue unassigned: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] Issue labels updated: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] Issue labels cleared: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] Issue synchronized: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] Issue milestone: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] Issue clear milestone: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
	}

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text: title + "\r\n\r\n" + text,
			//Markdown:    "# " + title + "\n" + text,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: "view issue",
			SingleURL:   p.Issue.URL,
		},
	}, nil
}

func getDingtalkIssueCommentPayload(p *api.IssueCommentPayload) (*DingtalkPayload, error) {
	title := fmt.Sprintf("#%d %s", p.Issue.Index, p.Issue.Title)
	url := fmt.Sprintf("%s/issues/%d#%s", p.Repository.HTMLURL, p.Issue.Index, CommentHashTag(p.Comment.ID))
	var content string
	switch p.Action {
	case api.HookIssueCommentCreated:
		title = "New comment: " + title
		content = p.Comment.Body
	case api.HookIssueCommentEdited:
		title = "Comment edited: " + title
		content = p.Comment.Body
	case api.HookIssueCommentDeleted:
		title = "Comment deleted: " + title
		url = fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Issue.Index)
		content = p.Comment.Body
	}

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text:        title + "\r\n\r\n" + content,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: "view issue comment",
			SingleURL:   url,
		},
	}, nil
}

func getDingtalkPullRequestPayload(p *api.PullRequestPayload) (*DingtalkPayload, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] Pull request opened: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			title = fmt.Sprintf("[%s] Pull request merged: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		} else {
			title = fmt.Sprintf("[%s] Pull request closed: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		}
		text = p.PullRequest.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] Pull request re-opened: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] Pull request edited: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = user.UserName
		}
		title = fmt.Sprintf("[%s] Pull request assigned to %s: #%d %s", p.Repository.FullName,
			strings.Join(list, ", "),
			p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] Pull request unassigned: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] Pull request labels updated: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] Pull request labels cleared: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] Pull request synchronized: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] Pull request milestone: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] Pull request clear milestone: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	}

	return &DingtalkPayload{
		MsgType: "actionCard",
		ActionCard: dingtalk.ActionCard{
			Text: title + "\r\n\r\n" + text,
			//Markdown:    "# " + title + "\n" + text,
			Title:       title,
			HideAvatar:  "0",
			SingleTitle: "view pull request",
			SingleURL:   p.PullRequest.HTMLURL,
		},
	}, nil
}

func getDingtalkPullRequestApprovalPayload(p *api.PullRequestPayload, event HookEventType) (*DingtalkPayload, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body

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
	var title, url string
	switch p.Action {
	case api.HookReleasePublished:
		title = fmt.Sprintf("[%s] Release created", p.Release.TagName)
		url = p.Release.URL
		return &DingtalkPayload{
			MsgType: "actionCard",
			ActionCard: dingtalk.ActionCard{
				Text:        title,
				Title:       title,
				HideAvatar:  "0",
				SingleTitle: "view release",
				SingleURL:   url,
			},
		}, nil
	case api.HookReleaseUpdated:
		title = fmt.Sprintf("[%s] Release updated", p.Release.TagName)
		url = p.Release.URL
		return &DingtalkPayload{
			MsgType: "actionCard",
			ActionCard: dingtalk.ActionCard{
				Text:        title,
				Title:       title,
				HideAvatar:  "0",
				SingleTitle: "view release",
				SingleURL:   url,
			},
		}, nil

	case api.HookReleaseDeleted:
		title = fmt.Sprintf("[%s] Release deleted", p.Release.TagName)
		url = p.Release.URL
		return &DingtalkPayload{
			MsgType: "actionCard",
			ActionCard: dingtalk.ActionCard{
				Text:        title,
				Title:       title,
				HideAvatar:  "0",
				SingleTitle: "view release",
				SingleURL:   url,
			},
		}, nil
	}

	return nil, nil
}

// GetDingtalkPayload converts a ding talk webhook into a DingtalkPayload
func GetDingtalkPayload(p api.Payloader, event HookEventType, meta string) (*DingtalkPayload, error) {
	s := new(DingtalkPayload)

	switch event {
	case HookEventCreate:
		return getDingtalkCreatePayload(p.(*api.CreatePayload))
	case HookEventDelete:
		return getDingtalkDeletePayload(p.(*api.DeletePayload))
	case HookEventFork:
		return getDingtalkForkPayload(p.(*api.ForkPayload))
	case HookEventIssues:
		return getDingtalkIssuesPayload(p.(*api.IssuePayload))
	case HookEventIssueComment:
		return getDingtalkIssueCommentPayload(p.(*api.IssueCommentPayload))
	case HookEventPush:
		return getDingtalkPushPayload(p.(*api.PushPayload))
	case HookEventPullRequest:
		return getDingtalkPullRequestPayload(p.(*api.PullRequestPayload))
	case HookEventPullRequestApproved, HookEventPullRequestRejected, HookEventPullRequestComment:
		return getDingtalkPullRequestApprovalPayload(p.(*api.PullRequestPayload), event)
	case HookEventRepository:
		return getDingtalkRepositoryPayload(p.(*api.RepositoryPayload))
	case HookEventRelease:
		return getDingtalkReleasePayload(p.(*api.ReleasePayload))
	}

	return s, nil
}
