// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

type (
	// DiscordEmbedFooter for Embed Footer Structure.
	DiscordEmbedFooter struct {
		Text string `json:"text"`
	}

	// DiscordEmbedAuthor for Embed Author Structure
	DiscordEmbedAuthor struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		IconURL string `json:"icon_url"`
	}

	// DiscordEmbedField for Embed Field Structure
	DiscordEmbedField struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	// DiscordEmbed is for Embed Structure
	DiscordEmbed struct {
		Title       string              `json:"title"`
		Description string              `json:"description"`
		URL         string              `json:"url"`
		Color       int                 `json:"color"`
		Footer      DiscordEmbedFooter  `json:"footer"`
		Author      DiscordEmbedAuthor  `json:"author"`
		Fields      []DiscordEmbedField `json:"fields"`
	}

	// DiscordPayload represents
	DiscordPayload struct {
		Wait      bool           `json:"wait"`
		Content   string         `json:"content"`
		Username  string         `json:"username"`
		AvatarURL string         `json:"avatar_url,omitempty"`
		TTS       bool           `json:"tts"`
		Embeds    []DiscordEmbed `json:"embeds"`
	}

	// DiscordMeta contains the discord metadata
	DiscordMeta struct {
		Username string `json:"username"`
		IconURL  string `json:"icon_url"`
	}
)

// GetDiscordHook returns discord metadata
func GetDiscordHook(w *webhook_model.Webhook) *DiscordMeta {
	s := &DiscordMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetDiscordHook(%d): %v", w.ID, err)
	}
	return s
}

func color(clr string) int {
	if clr != "" {
		clr = strings.TrimLeft(clr, "#")
		if s, err := strconv.ParseInt(clr, 16, 32); err == nil {
			return int(s)
		}
	}

	return 0
}

var (
	greenColor       = color("1ac600")
	greenColorLight  = color("bfe5bf")
	yellowColor      = color("ffd930")
	greyColor        = color("4f545c")
	purpleColor      = color("7289da")
	orangeColor      = color("eb6420")
	orangeColorLight = color("e68d60")
	redColor         = color("ff3232")
)

type discordConvertor struct {
	Username  string
	AvatarURL string
}

// Create implements PayloadConvertor Create method
func (d discordConvertor) Create(p *api.CreatePayload) (DiscordPayload, error) {
	// created tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return d.createPayload(p.Sender, title, "", p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName), greenColor), nil
}

// Delete implements PayloadConvertor Delete method
func (d discordConvertor) Delete(p *api.DeletePayload) (DiscordPayload, error) {
	// deleted tag/branch
	refName := git.RefName(p.Ref).ShortName()
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return d.createPayload(p.Sender, title, "", p.Repo.HTMLURL+"/src/"+util.PathEscapeSegments(refName), redColor), nil
}

// Fork implements PayloadConvertor Fork method
func (d discordConvertor) Fork(p *api.ForkPayload) (DiscordPayload, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return d.createPayload(p.Sender, title, "", p.Repo.HTMLURL, greenColor), nil
}

// Push implements PayloadConvertor Push method
func (d discordConvertor) Push(p *api.PushPayload) (DiscordPayload, error) {
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
		titleLink = p.Repo.HTMLURL + "/src/" + util.PathEscapeSegments(branchName)
	}

	title := fmt.Sprintf("[%s:%s] %s", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		// limit the commit message display to just the summary, otherwise it would be hard to read
		message := strings.TrimRight(strings.SplitN(commit.Message, "\n", 2)[0], "\r")

		// a limit of 50 is set because GitHub does the same
		if utf8.RuneCountInString(message) > 50 {
			message = fmt.Sprintf("%.47s...", message)
		}
		text += fmt.Sprintf("[%s](%s) %s - %s", commit.ID[:7], commit.URL, message, commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}

	return d.createPayload(p.Sender, title, text, titleLink, greenColor), nil
}

// Issue implements PayloadConvertor Issue method
func (d discordConvertor) Issue(p *api.IssuePayload) (DiscordPayload, error) {
	title, _, extraMarkdown, color := getIssuesPayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, title, extraMarkdown, p.Issue.HTMLURL, color), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (d discordConvertor) IssueComment(p *api.IssueCommentPayload) (DiscordPayload, error) {
	title, _, color := getIssueCommentPayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, title, p.Comment.Body, p.Comment.HTMLURL, color), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (d discordConvertor) PullRequest(p *api.PullRequestPayload) (DiscordPayload, error) {
	title, _, extraMarkdown, color := getPullRequestPayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, title, extraMarkdown, p.PullRequest.HTMLURL, color), nil
}

// Review implements PayloadConvertor Review method
func (d discordConvertor) Review(p *api.PullRequestPayload, event webhook_module.HookEventType) (DiscordPayload, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return DiscordPayload{}, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content

		switch event {
		case webhook_module.HookEventPullRequestReviewApproved:
			color = greenColor
		case webhook_module.HookEventPullRequestReviewRejected:
			color = redColor
		case webhook_module.HookEventPullRequestReviewComment:
			color = greyColor
		default:
			color = yellowColor
		}
	}

	return d.createPayload(p.Sender, title, text, p.PullRequest.HTMLURL, color), nil
}

// Repository implements PayloadConvertor Repository method
func (d discordConvertor) Repository(p *api.RepositoryPayload) (DiscordPayload, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		color = greenColor
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		color = redColor
	}

	return d.createPayload(p.Sender, title, "", url, color), nil
}

// Wiki implements PayloadConvertor Wiki method
func (d discordConvertor) Wiki(p *api.WikiPayload) (DiscordPayload, error) {
	text, color, _ := getWikiPayloadInfo(p, noneLinkFormatter, false)
	htmlLink := p.Repository.HTMLURL + "/wiki/" + url.PathEscape(p.Page)

	var description string
	if p.Action != api.HookWikiDeleted {
		description = p.Comment
	}

	return d.createPayload(p.Sender, text, description, htmlLink, color), nil
}

// Release implements PayloadConvertor Release method
func (d discordConvertor) Release(p *api.ReleasePayload) (DiscordPayload, error) {
	text, color := getReleasePayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, text, p.Release.Note, p.Release.HTMLURL, color), nil
}

func (d discordConvertor) Package(p *api.PackagePayload) (DiscordPayload, error) {
	text, color := getPackagePayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, text, "", p.Package.HTMLURL, color), nil
}

func (d discordConvertor) Status(p *api.CommitStatusPayload) (DiscordPayload, error) {
	text, color := getStatusPayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, text, "", p.TargetURL, color), nil
}

func newDiscordRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	meta := &DiscordMeta{}
	if err := json.Unmarshal([]byte(w.Meta), meta); err != nil {
		return nil, nil, fmt.Errorf("newDiscordRequest meta json: %w", err)
	}
	var pc payloadConvertor[DiscordPayload] = discordConvertor{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
	}
	return newJSONRequest(pc, w, t, true)
}

func init() {
	RegisterWebhookRequester(webhook_module.DISCORD, newDiscordRequest)
}

func parseHookPullRequestEventType(event webhook_module.HookEventType) (string, error) {
	switch event {
	case webhook_module.HookEventPullRequestReviewApproved:
		return "approved", nil
	case webhook_module.HookEventPullRequestReviewRejected:
		return "rejected", nil
	case webhook_module.HookEventPullRequestReviewComment:
		return "comment", nil
	default:
		return "", errors.New("unknown event type")
	}
}

func (d discordConvertor) createPayload(s *api.User, title, text, url string, color int) DiscordPayload {
	return DiscordPayload{
		Username:  d.Username,
		AvatarURL: d.AvatarURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: text,
				URL:         url,
				Color:       color,
				Author: DiscordEmbedAuthor{
					Name:    s.UserName,
					URL:     setting.AppURL + s.UserName,
					IconURL: s.AvatarURL,
				},
			},
		},
	}
}
