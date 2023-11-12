// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
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
		ThreadID string `json:"thread_id"`
	}
)

// GetTelegramHook returns telegram metadata
func GetTelegramHook(w *webhook_model.Webhook) *TelegramMeta {
	s := &TelegramMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetTelegramHook(%d): %v", w.ID, err)
	}
	return s
}

var _ PayloadConvertor = &TelegramPayload{}

// JSONPayload Marshals the TelegramPayload to json
func (t *TelegramPayload) JSONPayload() ([]byte, error) {
	t.ParseMode = "HTML"
	t.DisableWebPreview = true
	t.Message = markup.Sanitize(t.Message)
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// Create implements PayloadConvertor Create method
func (t *TelegramPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf(`[<a href="%s">%s</a>] %s <a href="%s">%s</a> created`, p.Repo.HTMLURL, p.Repo.FullName, p.RefType,
		p.Repo.HTMLURL+"/src/"+refName, refName)

	return createTelegramPayload(title), nil
}

// Delete implements PayloadConvertor Delete method
func (t *TelegramPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf(`[<a href="%s">%s</a>] %s <a href="%s">%s</a> deleted`, p.Repo.HTMLURL, p.Repo.FullName, p.RefType,
		p.Repo.HTMLURL+"/src/"+refName, refName)

	return createTelegramPayload(title), nil
}

// Fork implements PayloadConvertor Fork method
func (t *TelegramPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	title := fmt.Sprintf(`%s is forked to <a href="%s">%s</a>`, p.Forkee.FullName, p.Repo.HTMLURL, p.Repo.FullName)

	return createTelegramPayload(title), nil
}

// Push implements PayloadConvertor Push method
func (t *TelegramPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	var (
		branchName = git.RefName(p.Ref).ShortName()
		commitDesc string
	)

	var titleLink string
	if p.TotalCommits == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
	} else {
		commitDesc = fmt.Sprintf("%d new commits", p.TotalCommits)
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

	return createTelegramPayload(title + "\n" + text), nil
}

// Issue implements PayloadConvertor Issue method
func (t *TelegramPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	text, _, attachmentText, _ := getIssuesPayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayload(text + "\n\n" + attachmentText), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (t *TelegramPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	text, _, _ := getIssueCommentPayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayload(text + "\n" + p.Comment.Body), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (t *TelegramPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	text, _, attachmentText, _ := getPullRequestPayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayload(text + "\n" + attachmentText), nil
}

// Review implements PayloadConvertor Review method
func (t *TelegramPayload) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (api.Payloader, error) {
	var text, attachmentText string
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		attachmentText = p.Review.Content
	}

	return createTelegramPayload(text + "\n" + attachmentText), nil
}

// Repository implements PayloadConvertor Repository method
func (t *TelegramPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	var title string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf(`[<a href="%s">%s</a>] Repository created`, p.Repository.HTMLURL, p.Repository.FullName)
		return createTelegramPayload(title), nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return createTelegramPayload(title), nil
	}
	return nil, nil
}

// Wiki implements PayloadConvertor Wiki method
func (t *TelegramPayload) Wiki(p *api.WikiPayload) (api.Payloader, error) {
	text, _, _ := getWikiPayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayload(text), nil
}

// Release implements PayloadConvertor Release method
func (t *TelegramPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, _ := getReleasePayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayload(text), nil
}

func (t *TelegramPayload) Package(p *api.PackagePayload) (api.Payloader, error) {
	text, _ := getPackagePayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayload(text), nil
}

// GetTelegramPayload converts a telegram webhook into a TelegramPayload
func GetTelegramPayload(p api.Payloader, event webhook_module.HookEventType, _ string) (api.Payloader, error) {
	return convertPayloader(new(TelegramPayload), p, event)
}

func createTelegramPayload(message string) *TelegramPayload {
	return &TelegramPayload{
		Message: strings.TrimSpace(message),
	}
}
