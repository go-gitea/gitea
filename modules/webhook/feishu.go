// Copyright 2020 The Gitea Authors. All rights reserved.
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
)

type (
	// FeishuPayload represents
	FeishuPayload struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	}
)

// SetSecret sets the Feishu secret
func (p *FeishuPayload) SetSecret(_ string) {}

// JSONPayload Marshals the FeishuPayload to json
func (p *FeishuPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func getFeishuCreatePayload(p *api.CreatePayload) (*FeishuPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return &FeishuPayload{
		Text:  title,
		Title: title,
	}, nil
}

func getFeishuDeletePayload(p *api.DeletePayload) (*FeishuPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return &FeishuPayload{
		Text:  title,
		Title: title,
	}, nil
}

func getFeishuForkPayload(p *api.ForkPayload) (*FeishuPayload, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return &FeishuPayload{
		Text:  title,
		Title: title,
	}, nil
}

func getFeishuPushPayload(p *api.PushPayload) (*FeishuPayload, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

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

	return &FeishuPayload{
		Text:  text,
		Title: title,
	}, nil
}

func getFeishuIssuesPayload(p *api.IssuePayload) (*FeishuPayload, error) {
	text, issueTitle, attachmentText, _ := getIssuesPayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text + "\r\n\r\n" + attachmentText,
		Title: issueTitle,
	}, nil
}

func getFeishuIssueCommentPayload(p *api.IssueCommentPayload) (*FeishuPayload, error) {
	text, issueTitle, _ := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text + "\r\n\r\n" + p.Comment.Body,
		Title: issueTitle,
	}, nil
}

func getFeishuPullRequestPayload(p *api.PullRequestPayload) (*FeishuPayload, error) {
	text, issueTitle, attachmentText, _ := getPullRequestPayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text + "\r\n\r\n" + attachmentText,
		Title: issueTitle,
	}, nil
}

func getFeishuPullRequestApprovalPayload(p *api.PullRequestPayload, event models.HookEventType) (*FeishuPayload, error) {
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

	return &FeishuPayload{
		Text:  title + "\r\n\r\n" + text,
		Title: title,
	}, nil
}

func getFeishuRepositoryPayload(p *api.RepositoryPayload) (*FeishuPayload, error) {
	var title string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return &FeishuPayload{
			Text:  title,
			Title: title,
		}, nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return &FeishuPayload{
			Title: title,
			Text:  title,
		}, nil
	}

	return nil, nil
}

func getFeishuReleasePayload(p *api.ReleasePayload) (*FeishuPayload, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return &FeishuPayload{
		Text:  text,
		Title: text,
	}, nil
}

// GetFeishuPayload converts a ding talk webhook into a FeishuPayload
func GetFeishuPayload(p api.Payloader, event models.HookEventType, meta string) (*FeishuPayload, error) {
	s := new(FeishuPayload)

	switch event {
	case models.HookEventCreate:
		return getFeishuCreatePayload(p.(*api.CreatePayload))
	case models.HookEventDelete:
		return getFeishuDeletePayload(p.(*api.DeletePayload))
	case models.HookEventFork:
		return getFeishuForkPayload(p.(*api.ForkPayload))
	case models.HookEventIssues:
		return getFeishuIssuesPayload(p.(*api.IssuePayload))
	case models.HookEventIssueComment, models.HookEventPullRequestComment:
		pl, ok := p.(*api.IssueCommentPayload)
		if ok {
			return getFeishuIssueCommentPayload(pl)
		}
		return getFeishuPullRequestPayload(p.(*api.PullRequestPayload))
	case models.HookEventPush:
		return getFeishuPushPayload(p.(*api.PushPayload))
	case models.HookEventPullRequest:
		return getFeishuPullRequestPayload(p.(*api.PullRequestPayload))
	case models.HookEventPullRequestReviewApproved, models.HookEventPullRequestReviewRejected:
		return getFeishuPullRequestApprovalPayload(p.(*api.PullRequestPayload), event)
	case models.HookEventRepository:
		return getFeishuRepositoryPayload(p.(*api.RepositoryPayload))
	case models.HookEventRelease:
		return getFeishuReleasePayload(p.(*api.ReleasePayload))
	}

	return s, nil
}
