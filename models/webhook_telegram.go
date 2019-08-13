// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/markup"
	api "code.gitea.io/gitea/modules/structs"
)

type (
	// TelegramPayload represents
	TelegramPayload struct {
		Message           string `json:"text"`
		ParseMode         string `json:"parse_mode"`
		DisableWebPreview bool   `json:"disable_web_page_preview"`
	}

	// TelegramMeta contains the telegram metadata
	TelegramMeta struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
	}
)

// SetSecret sets the telegram secret
func (p *TelegramPayload) SetSecret(_ string) {}

// JSONPayload Marshals the TelegramPayload to json
func (p *TelegramPayload) JSONPayload() ([]byte, error) {
	p.ParseMode = "HTML"
	p.DisableWebPreview = true
	p.Message = markup.Sanitize(p.Message)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func getTelegramCreatePayload(p *api.CreatePayload) (*TelegramPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf(`[<a href="%s">%s</a>] %s <a href="%s">%s</a> created`, p.Repo.HTMLURL, p.Repo.FullName, p.RefType,
		p.Repo.HTMLURL+"/src/"+refName, refName)

	return &TelegramPayload{
		Message: title,
	}, nil
}

func getTelegramDeletePayload(p *api.DeletePayload) (*TelegramPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf(`[<a href="%s">%s</a>] %s <a href="%s">%s</a> deleted`, p.Repo.HTMLURL, p.Repo.FullName, p.RefType,
		p.Repo.HTMLURL+"/src/"+refName, refName)

	return &TelegramPayload{
		Message: title,
	}, nil
}

func getTelegramForkPayload(p *api.ForkPayload) (*TelegramPayload, error) {
	title := fmt.Sprintf(`%s is forked to <a href="%s">%s</a>`, p.Forkee.FullName, p.Repo.HTMLURL, p.Repo.FullName)

	return &TelegramPayload{
		Message: title,
	}, nil
}

func getTelegramPushPayload(p *api.PushPayload) (*TelegramPayload, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

	var titleLink string
	if len(p.Commits) == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
	} else {
		commitDesc = fmt.Sprintf("%d new commits", len(p.Commits))
		titleLink = p.CompareURL
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + branchName
	}
	title := fmt.Sprintf(`[<a href="%s">%s</a>:<a href="%s">%s</a>] %s`, p.Repo.HTMLURL, p.Repo.FullName, titleLink, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = " - " + commit.Author.Name
		}
		text += fmt.Sprintf(`[<a href="%s">%s</a>] %s`, commit.URL, commit.ID[:7],
			strings.TrimRight(commit.Message, "\r\n")) + authorName
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}

	return &TelegramPayload{
		Message: title + "\n" + text,
	}, nil
}

func getTelegramIssuesPayload(p *api.IssuePayload) (*TelegramPayload, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue opened: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueClosed:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue closed: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue re-opened: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueEdited:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue edited: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueAssigned:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue assigned to %s: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.Assignee.UserName, p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueUnassigned:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue unassigned: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue labels updated: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue labels cleared: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueSynchronized:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue synchronized: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueMilestoned:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue milestone: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Issue clear milestone: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.Issue.URL, p.Index, p.Issue.Title)
		text = p.Issue.Body
	}

	return &TelegramPayload{
		Message: title + "\n\n" + text,
	}, nil
}

func getTelegramIssueCommentPayload(p *api.IssueCommentPayload) (*TelegramPayload, error) {
	url := fmt.Sprintf("%s/issues/%d#%s", p.Repository.HTMLURL, p.Issue.Index, CommentHashTag(p.Comment.ID))
	title := fmt.Sprintf(`<a href="%s">#%d %s</a>`, url, p.Issue.Index, html.EscapeString(p.Issue.Title))
	var text string
	switch p.Action {
	case api.HookIssueCommentCreated:
		text = "New comment: " + title
		text += p.Comment.Body
	case api.HookIssueCommentEdited:
		text = "Comment edited: " + title
		text += p.Comment.Body
	case api.HookIssueCommentDeleted:
		text = "Comment deleted: " + title
		text += p.Comment.Body
	}

	return &TelegramPayload{
		Message: title + "\n" + text,
	}, nil
}

func getTelegramPullRequestPayload(p *api.PullRequestPayload) (*TelegramPayload, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request opened: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request merged: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
				p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		} else {
			title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request closed: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
				p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		}
		text = p.PullRequest.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request re-opened: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueEdited:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request edited: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueAssigned:
		list, err := MakeAssigneeList(&Issue{ID: p.PullRequest.ID})
		if err != nil {
			return &TelegramPayload{}, err
		}
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request assigned to %s: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			list, p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueUnassigned:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request unassigned: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request labels updated: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request labels cleared: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueSynchronized:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request synchronized: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueMilestoned:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request milestone: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Pull request clear milestone: <a href="%s">#%d %s</a>`, p.Repository.HTMLURL, p.Repository.FullName,
			p.PullRequest.HTMLURL, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
	}

	return &TelegramPayload{
		Message: title + "\n" + text,
	}, nil
}

func getTelegramRepositoryPayload(p *api.RepositoryPayload) (*TelegramPayload, error) {
	var title string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Repository created`, p.Repository.HTMLURL, p.Repository.FullName)
		return &TelegramPayload{
			Message: title,
		}, nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return &TelegramPayload{
			Message: title,
		}, nil
	}
	return nil, nil
}

func getTelegramReleasePayload(p *api.ReleasePayload) (*TelegramPayload, error) {
	var title, url string
	switch p.Action {
	case api.HookReleasePublished:
		title = fmt.Sprintf("[%s] Release created", p.Release.TagName)
		url = p.Release.URL
		return &TelegramPayload{
			Message: title + "\n" + url,
		}, nil
	case api.HookReleaseUpdated:
		title = fmt.Sprintf("[%s] Release updated", p.Release.TagName)
		url = p.Release.URL
		return &TelegramPayload{
			Message: title + "\n" + url,
		}, nil

	case api.HookReleaseDeleted:
		title = fmt.Sprintf("[%s] Release deleted", p.Release.TagName)
		url = p.Release.URL
		return &TelegramPayload{
			Message: title + "\n" + url,
		}, nil
	}

	return nil, nil
}

// GetTelegramPayload converts a telegram webhook into a TelegramPayload
func GetTelegramPayload(p api.Payloader, event HookEventType, meta string) (*TelegramPayload, error) {
	s := new(TelegramPayload)

	switch event {
	case HookEventCreate:
		return getTelegramCreatePayload(p.(*api.CreatePayload))
	case HookEventDelete:
		return getTelegramDeletePayload(p.(*api.DeletePayload))
	case HookEventFork:
		return getTelegramForkPayload(p.(*api.ForkPayload))
	case HookEventIssues:
		return getTelegramIssuesPayload(p.(*api.IssuePayload))
	case HookEventIssueComment:
		return getTelegramIssueCommentPayload(p.(*api.IssueCommentPayload))
	case HookEventPush:
		return getTelegramPushPayload(p.(*api.PushPayload))
	case HookEventPullRequest:
		return getTelegramPullRequestPayload(p.(*api.PullRequestPayload))
	case HookEventRepository:
		return getTelegramRepositoryPayload(p.(*api.RepositoryPayload))
	case HookEventRelease:
		return getTelegramReleasePayload(p.(*api.ReleasePayload))
	}

	return s, nil
}
