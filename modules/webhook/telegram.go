// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
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

// GetTelegramHook returns telegram metadata
func GetTelegramHook(w *models.Webhook) *TelegramMeta {
	s := &TelegramMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetTelegramHook(%d): %v", w.ID, err)
	}
	return s
}

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
	text, _, attachmentText, _ := getIssuesPayloadInfo(p, htmlLinkFormatter, true)

	return &TelegramPayload{
		Message: text + "\n\n" + attachmentText,
	}, nil
}

func getTelegramIssueCommentPayload(p *api.IssueCommentPayload) (*TelegramPayload, error) {
	text, _, _ := getIssueCommentPayloadInfo(p, htmlLinkFormatter, true)

	return &TelegramPayload{
		Message: text + "\n" + p.Comment.Body,
	}, nil
}

func getTelegramPullRequestPayload(p *api.PullRequestPayload) (*TelegramPayload, error) {
	text, _, attachmentText, _ := getPullRequestPayloadInfo(p, htmlLinkFormatter, true)

	return &TelegramPayload{
		Message: text + "\n" + attachmentText,
	}, nil
}

func getTelegramPullRequestApprovalPayload(p *api.PullRequestPayload, event models.HookEventType) (*TelegramPayload, error) {
	var text, attachmentText string
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		attachmentText = p.Review.Content

	}

	return &TelegramPayload{
		Message: text + "\n" + attachmentText,
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
	text, _ := getReleasePayloadInfo(p, htmlLinkFormatter, true)

	return &TelegramPayload{
		Message: text + "\n",
	}, nil
}

// GetTelegramPayload converts a telegram webhook into a TelegramPayload
func GetTelegramPayload(p api.Payloader, event models.HookEventType, meta string) (*TelegramPayload, error) {
	s := new(TelegramPayload)

	switch event {
	case models.HookEventCreate:
		return getTelegramCreatePayload(p.(*api.CreatePayload))
	case models.HookEventDelete:
		return getTelegramDeletePayload(p.(*api.DeletePayload))
	case models.HookEventFork:
		return getTelegramForkPayload(p.(*api.ForkPayload))
	case models.HookEventIssues:
		return getTelegramIssuesPayload(p.(*api.IssuePayload))
	case models.HookEventIssueComment:
		return getTelegramIssueCommentPayload(p.(*api.IssueCommentPayload))
	case models.HookEventPush:
		return getTelegramPushPayload(p.(*api.PushPayload))
	case models.HookEventPullRequest:
		return getTelegramPullRequestPayload(p.(*api.PullRequestPayload))
	case models.HookEventPullRequestRejected, models.HookEventPullRequestApproved, models.HookEventPullRequestComment:
		return getTelegramPullRequestApprovalPayload(p.(*api.PullRequestPayload), event)
	case models.HookEventRepository:
		return getTelegramRepositoryPayload(p.(*api.RepositoryPayload))
	case models.HookEventRelease:
		return getTelegramReleasePayload(p.(*api.ReleasePayload))
	}

	return s, nil
}
