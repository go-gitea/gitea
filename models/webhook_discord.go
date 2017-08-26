package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/git"
	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/modules/setting"
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
		Color    int    `json:"color"`
	}
)

// SetSecret sets the slack secret
func (p *DiscordPayload) SetSecret(_ string) {}

// JSONPayload Marshals the SlackPayload to json
func (p *DiscordPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func replaceBadCharsForDiscord(in string) string {
	return strings.NewReplacer("[", "", "]", ":", ":", "/").Replace(in)
}

func getDiscordCreatePayload(p *api.CreatePayload, meta *DiscordMeta) (*DiscordPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)

	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.Name)
	refLink := SlackLinkFormatter(p.Repo.HTMLURL+"/src/"+refName, refName)

	format := "[%s:%s] %s created by %s"
	format = replaceBadCharsForDiscord(format)
	text := fmt.Sprintf(format, repoLink, refLink, p.RefType, p.Sender.UserName)

	var username = meta.Username
	if username == "" {
		username = "Gitea"
	}

	return &DiscordPayload{
		Content:   text,
		Username:  username,
		AvatarURL: meta.IconURL,
	}, nil
}

func getDiscordPushPayload(p *api.PushPayload, meta *DiscordMeta) (*DiscordPayload, error) {
	// n new commits
	var (
		branchName   = git.RefEndName(p.Ref)
		commitDesc   string
		commitString string
	)

	if len(p.Commits) == 1 {
		commitDesc = "1 new commit"
	} else {
		commitDesc = fmt.Sprintf("%d new commits", len(p.Commits))
	}
	if len(p.CompareURL) > 0 {
		commitString = SlackLinkFormatter(p.CompareURL, commitDesc)
	} else {
		commitString = commitDesc
	}

	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.Name)
	branchLink := SlackLinkFormatter(p.Repo.HTMLURL+"/src/"+branchName, branchName)

	format := "[%s:%s] %s pushed by %s"
	format = replaceBadCharsForDiscord(format)
	text := fmt.Sprintf(format, repoLink, branchLink, commitString, p.Pusher.UserName)

	var attachmentText string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		attachmentText += fmt.Sprintf("%s: %s - %s", SlackLinkFormatter(commit.URL, commit.ID[:7]), SlackShortTextFormatter(commit.Message), SlackTextFormatter(commit.Author.Name))
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			attachmentText += "\n"
		}
	}

	var username = meta.Username
	if username == "" {
		username = "Gitea"
	}

	return &DiscordPayload{
		//Content:   text,
		Username:  username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       text,
				Description: attachmentText,
				//URL:         branchLink, // FIXME
				Color: meta.Color,
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
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	titleLink := SlackLinkFormatter(fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index),
		fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title))
	var text, title string
	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Pull request submitted by %s", p.Repository.FullName, senderLink)
		title = titleLink
		//attachmentText = SlackTextFormatter(p.PullRequest.Body)
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			text = fmt.Sprintf("[%s] Pull request merged: %s by %s", p.Repository.FullName, titleLink, senderLink)
		} else {
			text = fmt.Sprintf("[%s] Pull request closed: %s by %s", p.Repository.FullName, titleLink, senderLink)
		}
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Pull request re-opened: %s by %s", p.Repository.FullName, titleLink, senderLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Pull request edited: %s by %s", p.Repository.FullName, titleLink, senderLink)
		//attachmentText = SlackTextFormatter(p.PullRequest.Body)
	case api.HookIssueAssigned:
		text = fmt.Sprintf("[%s] Pull request assigned to %s: %s by %s", p.Repository.FullName,
			SlackLinkFormatter(setting.AppURL+p.PullRequest.Assignee.UserName, p.PullRequest.Assignee.UserName),
			titleLink, senderLink)
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Pull request unassigned: %s by %s", p.Repository.FullName, titleLink, senderLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Pull request labels updated: %s by %s", p.Repository.FullName, titleLink, senderLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Pull request labels cleared: %s by %s", p.Repository.FullName, titleLink, senderLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Pull request synchronized: %s by %s", p.Repository.FullName, titleLink, senderLink)
	}

	var username = meta.Username
	if username == "" {
		username = "Gitea"
	}

	return &DiscordPayload{
		//Content:   text,
		Username:  username,
		AvatarURL: meta.IconURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: text,
				URL:         p.PullRequest.HTMLURL,
				Color:       meta.Color,
				Author: DiscordEmbedAuthor{
					Name:    p.Sender.UserName,
					URL:     setting.AppURL + p.Sender.UserName,
					IconURL: p.Sender.AvatarURL,
				},
			},
		},
	}, nil
}

// GetDiscordPayload converts a slack webhook into a SlackPayload
func GetDiscordPayload(p api.Payloader, event HookEventType, meta string) (*DiscordPayload, error) {
	s := new(DiscordPayload)

	discord := &DiscordMeta{}
	if err := json.Unmarshal([]byte(meta), &discord); err != nil {
		return s, errors.New("GetSlackPayload meta json:" + err.Error())
	}

	switch event {
	case HookEventCreate:
		return getDiscordCreatePayload(p.(*api.CreatePayload), discord)
	case HookEventPush:
		return getDiscordPushPayload(p.(*api.PushPayload), discord)
	case HookEventPullRequest:
		return getDiscordPullRequestPayload(p.(*api.PullRequestPayload), discord)
	}

	return s, nil
}
