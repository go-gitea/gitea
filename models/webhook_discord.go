package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"
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
	successColor = color("1ac600")
	warnColor    = color("ffd930")
	failedColor  = color("ff3232")
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
				Color: successColor,
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
				Color:       successColor,
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
		color = warnColor
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			title = fmt.Sprintf("[%s] Pull request merged: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
			color = successColor
		} else {
			title = fmt.Sprintf("[%s] Pull request closed: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
			color = failedColor
		}
		text = p.PullRequest.Body
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] Pull request re-opened: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] Pull request edited: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueAssigned:
		title = fmt.Sprintf("[%s] Pull request assigned to %s: #%d %s", p.Repository.FullName,
			p.PullRequest.Assignee.UserName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = successColor
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] Pull request unassigned: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] Pull request labels updated: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] Pull request labels cleared: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] Pull request synchronized: #%d %s", p.Repository.FullName, p.Index, p.PullRequest.Title)
		text = p.PullRequest.Body
		color = warnColor
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
		color = successColor
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		color = warnColor
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

// GetDiscordPayload converts a discord webhook into a DiscordPayload
func GetDiscordPayload(p api.Payloader, event HookEventType, meta string) (*DiscordPayload, error) {
	s := new(DiscordPayload)

	discord := &DiscordMeta{}
	if err := json.Unmarshal([]byte(meta), &discord); err != nil {
		return s, errors.New("GetDiscordPayload meta json:" + err.Error())
	}

	switch event {
	case HookEventCreate:
		return getDiscordCreatePayload(p.(*api.CreatePayload), discord)
	case HookEventPush:
		return getDiscordPushPayload(p.(*api.PushPayload), discord)
	case HookEventPullRequest:
		return getDiscordPullRequestPayload(p.(*api.PullRequestPayload), discord)
	case HookEventRepository:
		return getDiscordRepositoryPayload(p.(*api.RepositoryPayload), discord)
	}

	return s, nil
}
