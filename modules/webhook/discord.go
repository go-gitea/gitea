// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
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
		AvatarURL string         `json:"avatar_url"`
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
func GetDiscordHook(w *models.Webhook) *DiscordMeta {
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

// SetSecret sets the discord secret
func (p *DiscordPayload) SetSecret(_ string) {}

// JSONPayload Marshals the DiscordPayload to json
func (p *DiscordPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func getDiscordCreatePayload(p *api.CreatePayload, meta *DiscordMeta) (*DiscordPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title: title,
				URL:   p.Repo.HTMLURL + "/src/" + refName,
				Color: greenColor,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordDeletePayload(p *api.DeletePayload, meta *DiscordMeta) (*DiscordPayload, error) {
	// deleted tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title: title,
				URL:   p.Repo.HTMLURL + "/src/" + refName,
				Color: redColor,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordForkPayload(p *api.ForkPayload, meta *DiscordMeta) (*DiscordPayload, error) {
	// fork
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title: title,
				URL:   p.Repo.HTMLURL,
				Color: greenColor,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordPushPayload(p *api.PushPayload, meta *DiscordMeta) (*DiscordPayload, error) {
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

	title := fmt.Sprintf("[%s:%s] %s", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		text += fmt.Sprintf("[%s](%s) %s - %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n"), commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: text,
				URL:         titleLink,
				Color:       greenColor,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordIssuesPayload(p *api.IssuePayload, meta *DiscordMeta) (*DiscordPayload, error) {
	var text, title string
	var color int
	url := fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Issue.Index)
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] Issue opened: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = orangeColor
	case api.HookIssueClosed:
		title = fmt.Sprintf("[%s] Issue closed: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		color = redColor
		text = p.Issue.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] Issue re-opened: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] Issue edited: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	case api.HookIssueAssigned:
		title = fmt.Sprintf("[%s] Issue assigned to %s: #%d %s", p.Repository.FullName,
			p.Issue.Assignee.UserName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = greenColor
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] Issue unassigned: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] Issue labels updated: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] Issue labels cleared: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] Issue synchronized: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] Issue milestone: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] Issue clear milestone: #%d %s", p.Repository.FullName, p.Index, p.Issue.Title)
		text = p.Issue.Body
		color = yellowColor
	}

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: text,
				URL:         url,
				Color:       color,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordIssueCommentPayload(p *api.IssueCommentPayload, discord *DiscordMeta) (*DiscordPayload, error) {
	title := fmt.Sprintf("#%d: %s", p.Issue.Index, p.Issue.Title)
	url := fmt.Sprintf("%s/issues/%d#%s", p.Repository.HTMLURL, p.Issue.Index, models.CommentHashTag(p.Comment.ID))
	content := ""
	var color int
	switch p.Action {
	case api.HookIssueCommentCreated:
		if p.IsPull {
			title = "New comment on pull request " + title
			color = greenColorLight
		} else {
			title = "New comment on issue " + title
			color = orangeColorLight
		}
		content = p.Comment.Body
	case api.HookIssueCommentEdited:
		if p.IsPull {
			title = "Comment edited on pull request " + title
		} else {
			title = "Comment edited on issue " + title
		}
		content = p.Comment.Body
		color = yellowColor
	case api.HookIssueCommentDeleted:
		if p.IsPull {
			title = "Comment deleted on pull request " + title
		} else {
			title = "Comment deleted on issue " + title
		}
		url = fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Issue.Index)
		content = p.Comment.Body
		color = redColor
	}

	title = fmt.Sprintf("[%s] %s", p.Repository.FullName, title)

	return &DiscordPayload{
		Username:  discord.Username,
		AvatarURL: discord.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: content,
				URL:         url,
				Color:       color,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordPullRequestPayload(p *api.PullRequestPayload, meta *DiscordMeta) (*DiscordPayload, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] Pull request opened: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = greenColor
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			title = fmt.Sprintf("[%s] Pull request merged: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
			color = purpleColor
		} else {
			title = fmt.Sprintf("[%s] Pull request closed: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
			color = redColor
		}
		text = p.PullRequest.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] Pull request re-opened: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] Pull request edited: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = user.UserName
		}
		title = fmt.Sprintf("[%s] Pull request assigned to %s: #%d by %s", p.Repository.FullName,
			strings.Join(list, ", "),
			p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = greenColor
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] Pull request unassigned: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] Pull request labels updated: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] Pull request labels cleared: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] Pull request synchronized: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] Pull request milestone: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] Pull request clear milestone: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = yellowColor
	}

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: text,
				URL:         p.PullRequest.HTMLURL,
				Color:       color,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordPullRequestApprovalPayload(p *api.PullRequestPayload, meta *DiscordMeta, event models.HookEventType) (*DiscordPayload, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content

		switch event {
		case models.HookEventPullRequestApproved:
			color = greenColor
		case models.HookEventPullRequestRejected:
			color = redColor
		case models.HookEventPullRequestComment:
			color = greyColor
		default:
			color = yellowColor
		}
	}

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: text,
				URL:         p.PullRequest.HTMLURL,
				Color:       color,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordRepositoryPayload(p *api.RepositoryPayload, meta *DiscordMeta) (*DiscordPayload, error) {
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

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title: title,
				URL:   url,
				Color: color,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

func getDiscordReleasePayload(p *api.ReleasePayload, meta *DiscordMeta) (*DiscordPayload, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookReleasePublished:
		title = fmt.Sprintf("[%s] Release created", p.Release.TagName)
		url = p.Release.URL
		color = greenColor
	case api.HookReleaseUpdated:
		title = fmt.Sprintf("[%s] Release updated", p.Release.TagName)
		url = p.Release.URL
		color = yellowColor
	case api.HookReleaseDeleted:
		title = fmt.Sprintf("[%s] Release deleted", p.Release.TagName)
		url = p.Release.URL
		color = redColor
	}

	return &DiscordPayload{
		Username:  meta.Username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: p.Release.Note,
				URL:         url,
				Color:       color,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

// GetDiscordPayload converts a discord webhook into a DiscordPayload
func GetDiscordPayload(p api.Payloader, event models.HookEventType, meta string) (*DiscordPayload, error) {
	s := new(DiscordPayload)

	discord := &DiscordMeta{}
	if err := json.Unmarshal([]byte(meta), &discord); err != nil {
		return s, errors.New("GetDiscordPayload meta json:" + err.Error())
	}

	switch event {
	case models.HookEventCreate:
		return getDiscordCreatePayload(p.(*api.CreatePayload), discord)
	case models.HookEventDelete:
		return getDiscordDeletePayload(p.(*api.DeletePayload), discord)
	case models.HookEventFork:
		return getDiscordForkPayload(p.(*api.ForkPayload), discord)
	case models.HookEventIssues:
		return getDiscordIssuesPayload(p.(*api.IssuePayload), discord)
	case models.HookEventIssueComment:
		return getDiscordIssueCommentPayload(p.(*api.IssueCommentPayload), discord)
	case models.HookEventPush:
		return getDiscordPushPayload(p.(*api.PushPayload), discord)
	case models.HookEventPullRequest:
		return getDiscordPullRequestPayload(p.(*api.PullRequestPayload), discord)
	case models.HookEventPullRequestRejected, models.HookEventPullRequestApproved, models.HookEventPullRequestComment:
		return getDiscordPullRequestApprovalPayload(p.(*api.PullRequestPayload), discord, event)
	case models.HookEventRepository:
		return getDiscordRepositoryPayload(p.(*api.RepositoryPayload), discord)
	case models.HookEventRelease:
		return getDiscordReleasePayload(p.(*api.ReleasePayload), discord)
	}

	return s, nil
}

func parseHookPullRequestEventType(event models.HookEventType) (string, error) {

	switch event {

	case models.HookEventPullRequestApproved:
		return "approved", nil
	case models.HookEventPullRequestRejected:
		return "rejected", nil
	case models.HookEventPullRequestComment:
		return "comment", nil

	default:
		return "", errors.New("unknown event type")
	}
}
