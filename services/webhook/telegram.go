// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
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

type telegramConvertor struct{}

// Create implements PayloadConvertor Create method
func (t telegramConvertor) Create(p *api.CreatePayload) (TelegramPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf(`[%s] %s %s created`,
		htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName),
		html.EscapeString(p.RefType),
		htmlLinkFormatter(p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName), refName),
	)

	return createTelegramPayloadHTML(title), nil
}

// Delete implements PayloadConvertor Delete method
func (t telegramConvertor) Delete(p *api.DeletePayload) (TelegramPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf(`[%s] %s %s deleted`,
		htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName),
		html.EscapeString(p.RefType),
		htmlLinkFormatter(p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName), refName),
	)
	return createTelegramPayloadHTML(title), nil
}

// Fork implements PayloadConvertor Fork method
func (t telegramConvertor) Fork(p *api.ForkPayload) (TelegramPayload, error) {
	title := fmt.Sprintf(`%s is forked to %s`, html.EscapeString(p.Forkee.FullName), htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName))
	return createTelegramPayloadHTML(title), nil
}

// Push implements PayloadConvertor Push method
func (t telegramConvertor) Push(p *api.PushPayload) (TelegramPayload, error) {
	branchName := git.RefName(p.Ref).ShortName()
	var titleLink, commitDesc string
	if p.TotalCommits == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
	} else {
		commitDesc = fmt.Sprintf("%d new commits", p.TotalCommits)
		titleLink = p.CompareURL
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + util.PathEscapeSegments(branchName)
	}
	title := fmt.Sprintf(`[%s:%s] %s`, htmlLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName), htmlLinkFormatter(titleLink, branchName), html.EscapeString(commitDesc))

	var htmlCommits string
	for _, commit := range p.Commits {
		htmlCommits += fmt.Sprintf("\n[%s] %s", htmlLinkFormatter(commit.URL, commit.ID[:7]), html.EscapeString(strings.TrimRight(commit.Message, "\r\n")))
		if commit.Author != nil {
			htmlCommits += " - " + html.EscapeString(commit.Author.Name)
		}
	}
	return createTelegramPayloadHTML(title + htmlCommits), nil
}

// Issue implements PayloadConvertor Issue method
func (t telegramConvertor) Issue(p *api.IssuePayload) (TelegramPayload, error) {
	text, _, extraMarkdown, _ := getIssuesPayloadInfo(p, htmlLinkFormatter, true)
	// TODO: at the moment the markdown can't be rendered easily because it has context-aware links (eg: attachments)
	return createTelegramPayloadHTML(text + "\n\n" + html.EscapeString(extraMarkdown)), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (t telegramConvertor) IssueComment(p *api.IssueCommentPayload) (TelegramPayload, error) {
	text, _, _ := getIssueCommentPayloadInfo(p, htmlLinkFormatter, true)
	return createTelegramPayloadHTML(text + "\n" + html.EscapeString(p.Comment.Body)), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (t telegramConvertor) PullRequest(p *api.PullRequestPayload) (TelegramPayload, error) {
	text, _, extraMarkdown, _ := getPullRequestPayloadInfo(p, htmlLinkFormatter, true)
	return createTelegramPayloadHTML(text + "\n" + html.EscapeString(extraMarkdown)), nil
}

// Review implements PayloadConvertor Review method
func (t telegramConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (TelegramPayload, error) {
	var text, extraMarkdown string
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return TelegramPayload{}, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: #%d %s", html.EscapeString(p.Repository.FullName), html.EscapeString(action), p.Index, html.EscapeString(p.PullRequest.Title))
		extraMarkdown = p.Review.Content
	}

	return createTelegramPayloadHTML(text + "\n" + html.EscapeString(extraMarkdown)), nil
}

// Repository implements PayloadConvertor Repository method
func (t telegramConvertor) Repository(p *api.RepositoryPayload) (TelegramPayload, error) {
	var title string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf(`[%s] Repository created`, htmlLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName))
		return createTelegramPayloadHTML(title), nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", html.EscapeString(p.Repository.FullName))
		return createTelegramPayloadHTML(title), nil
	}
	return TelegramPayload{}, nil
}

// Wiki implements PayloadConvertor Wiki method
func (t telegramConvertor) Wiki(p *api.WikiPayload) (TelegramPayload, error) {
	text, _, _ := getWikiPayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayloadHTML(text), nil
}

// Release implements PayloadConvertor Release method
func (t telegramConvertor) Release(p *api.ReleasePayload) (TelegramPayload, error) {
	text, _ := getReleasePayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayloadHTML(text), nil
}

func (t telegramConvertor) Package(p *api.PackagePayload) (TelegramPayload, error) {
	text, _ := getPackagePayloadInfo(p, htmlLinkFormatter, true)

	return createTelegramPayloadHTML(text), nil
}

func createTelegramPayloadHTML(msgHTML string) TelegramPayload {
	// https://core.telegram.org/bots/api#formatting-options
	return TelegramPayload{
		Message:           strings.TrimSpace(markup.Sanitize(msgHTML)),
		ParseMode:         "HTML",
		DisableWebPreview: true,
	}
}

func newTelegramRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	var pc payloadConvertor[TelegramPayload] = telegramConvertor{}
	return newJSONRequest(pc, w, t, true)
}
