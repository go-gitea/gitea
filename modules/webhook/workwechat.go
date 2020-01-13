// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/git"
	api "code.gitea.io/sdk/gitea"
)

type (
	// Text message
	Text struct {
		Content string `json:"content"`
	}

	//TextCard message
	TextCard struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		URL         string `json:"url"`
		ButtonText  string `json:"btntxt"`
	}
	//WorkwechatPayload represents
	WorkwechatPayload struct {
		ChatID   string   `json:"chatid"`
		MsgType  string   `json:"msgtype"`
		Text     Text     `json:"text"`
		TextCard TextCard `json:"textcard"`
		Safe     int      `json:"safe"`
	}

	// WorkwechatMeta contains the work wechat metadata
	WorkwechatMeta struct {
		ChatID string `json:"chatid"`
	}
)

// SetSecret sets the workwechat secret
func (p *WorkwechatPayload) SetSecret(_ string) {}

// JSONPayload Marshals the WorkwechatPayload to json
func (p *WorkwechatPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func getWorkwechatCreatePayload(p *api.CreatePayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return &WorkwechatPayload{
		ChatID:  meta.ChatID,
		MsgType: "textcard",
		TextCard: TextCard{
			Title:       title,
			Description: title,
			ButtonText:  fmt.Sprintf("view ref %s", refName),
			URL:         p.Repo.HTMLURL + "/src/" + refName,
		},
	}, nil
}

func getWorkwechatDeletePayload(p *api.DeletePayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return &WorkwechatPayload{
		ChatID:  meta.ChatID,
		MsgType: "textcard",
		TextCard: TextCard{
			Title:       title,
			Description: title,
			ButtonText:  fmt.Sprintf("view ref %s", refName),
			URL:         p.Repo.HTMLURL + "/src/" + refName,
		},
	}, nil
}

func getWorkwechatForkPayload(p *api.ForkPayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return &WorkwechatPayload{
		ChatID:  meta.ChatID,
		MsgType: "textcard",
		TextCard: TextCard{
			Description: title,
			Title:       title,
			ButtonText:  fmt.Sprintf("view forked repo %s", p.Repo.FullName),
			URL:         p.Repo.HTMLURL,
		},
	}, nil
}

func getWorkwechatPushPayload(p *api.PushPayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
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

	return &WorkwechatPayload{
		ChatID:  meta.ChatID,
		MsgType: "textcard",
		TextCard: TextCard{
			Description: text,
			Title:       title,
			ButtonText:  linkText,
			URL:         titleLink,
		},
	}, nil
}

func getWorkwechatIssuesPayload(p *api.IssuePayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
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

	return &WorkwechatPayload{
		ChatID:  meta.ChatID,
		MsgType: "textcard",
		TextCard: TextCard{
			Description: title + "\r\n\r\n" + text,
			//Markdown:    "# " + title + "\n" + text,
			Title:      title,
			ButtonText: "view issue",
			URL:        p.Issue.URL,
		},
	}, nil
}

func getWorkwechatIssueCommentPayload(p *api.IssueCommentPayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
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

	return &WorkwechatPayload{
		ChatID:  meta.ChatID,
		MsgType: "textcard",
		TextCard: TextCard{
			Description: title + "\r\n\r\n" + content,
			Title:       title,
			ButtonText:  "view issue comment",
			URL:         url,
		},
	}, nil
}

func getWorkwechatPullRequestPayload(p *api.PullRequestPayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
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
		list, err := MakeAssigneeList(&Issue{ID: p.PullRequest.ID})
		if err != nil {
			return &WorkwechatPayload{}, err
		}
		title = fmt.Sprintf("[%s] Pull request assigned to %s: #%d %s", p.Repository.FullName,
			list, p.Index, p.PullRequest.Title)
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

	return &WorkwechatPayload{
		ChatID:  meta.ChatID,
		MsgType: "textcard",
		TextCard: TextCard{
			Description: title + "\r\n\r\n" + text,
			//Markdown:    "# " + title + "\n" + text,
			Title:      title,
			ButtonText: "view pull request",
			URL:        p.PullRequest.HTMLURL,
		},
	}, nil
}

func getWorkwechatRepositoryPayload(p *api.RepositoryPayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
	var title, url string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		return &WorkwechatPayload{
			ChatID:  meta.ChatID,
			MsgType: "textcard",
			TextCard: TextCard{
				Description: title,
				Title:       title,
				ButtonText:  "view repository",
				URL:         url,
			},
		}, nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return &WorkwechatPayload{
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

func getWorkwechatReleasePayload(p *api.ReleasePayload, meta *WorkwechatMeta) (*WorkwechatPayload, error) {
	var title, url string
	switch p.Action {
	case api.HookReleasePublished:
		title = fmt.Sprintf("[%s] Release created", p.Release.TagName)
		url = p.Release.URL
		return &WorkwechatPayload{
			ChatID:  meta.ChatID,
			MsgType: "textcard",
			TextCard: TextCard{
				Description: title,
				Title:       title,
				ButtonText:  "view release",
				URL:         url,
			},
		}, nil
	case api.HookReleaseUpdated:
		title = fmt.Sprintf("[%s] Release updated", p.Release.TagName)
		url = p.Release.URL
		return &WorkwechatPayload{
			ChatID:  meta.ChatID,
			MsgType: "textcard",
			TextCard: TextCard{
				Description: title,
				Title:       title,
				ButtonText:  "view release",
				URL:         url,
			},
		}, nil

	case api.HookReleaseDeleted:
		title = fmt.Sprintf("[%s] Release deleted", p.Release.TagName)
		url = p.Release.URL
		return &WorkwechatPayload{
			ChatID:  meta.ChatID,
			MsgType: "textcard",
			TextCard: TextCard{
				Description: title,
				Title:       title,
				ButtonText:  "view release",
				URL:         url,
			},
		}, nil
	}

	return nil, nil
}

// GetWorkwechatPayload converts a work wechat webhook into a WorkwechatPayload
func GetWorkwechatPayload(p api.Payloader, event HookEventType, meta string) (*WorkwechatPayload, error) {
	s := new(WorkwechatPayload)

	workwechatMeta := &WorkwechatMeta{}
	if err := json.Unmarshal([]byte(meta), &workwechatMeta); err != nil {
		return s, errors.New("GetWorkwechatPayload meta json:" + err.Error())
	}
	switch event {
	case HookEventCreate:
		return getWorkwechatCreatePayload(p.(*api.CreatePayload), workwechatMeta)
	case HookEventDelete:
		return getWorkwechatDeletePayload(p.(*api.DeletePayload), workwechatMeta)
	case HookEventFork:
		return getWorkwechatForkPayload(p.(*api.ForkPayload), workwechatMeta)
	case HookEventIssues:
		return getWorkwechatIssuesPayload(p.(*api.IssuePayload), workwechatMeta)
	case HookEventIssueComment:
		return getWorkwechatIssueCommentPayload(p.(*api.IssueCommentPayload), workwechatMeta)
	case HookEventPush:
		return getWorkwechatPushPayload(p.(*api.PushPayload), workwechatMeta)
	case HookEventPullRequest:
		return getWorkwechatPullRequestPayload(p.(*api.PullRequestPayload), workwechatMeta)
	case HookEventRepository:
		return getWorkwechatRepositoryPayload(p.(*api.RepositoryPayload), workwechatMeta)
	case HookEventRelease:
		return getWorkwechatReleasePayload(p.(*api.ReleasePayload), workwechatMeta)
	}

	return s, nil
}
