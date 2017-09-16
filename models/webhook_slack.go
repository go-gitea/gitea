// Copyright 2014 The Gogs Authors. All rights reserved.
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

	"code.gitea.io/gitea/modules/setting"
)

// SlackMeta contains the slack metadata
type SlackMeta struct {
	Channel  string `json:"channel"`
	Username string `json:"username"`
	IconURL  string `json:"icon_url"`
	Color    string `json:"color"`
}

// SlackPayload contains the information about the slack channel
type SlackPayload struct {
	Channel     string            `json:"channel"`
	Text        string            `json:"text"`
	Username    string            `json:"username"`
	IconURL     string            `json:"icon_url"`
	UnfurlLinks int               `json:"unfurl_links"`
	LinkNames   int               `json:"link_names"`
	Attachments []SlackAttachment `json:"attachments"`
}

// SlackAttachment contains the slack message
type SlackAttachment struct {
	Fallback string `json:"fallback"`
	Color    string `json:"color"`
	Title    string `json:"title"`
	Text     string `json:"text"`
}

// SetSecret sets the slack secret
func (p *SlackPayload) SetSecret(_ string) {}

// JSONPayload Marshals the SlackPayload to json
func (p *SlackPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// SlackTextFormatter replaces &, <, > with HTML characters
// see: https://api.slack.com/docs/formatting
func SlackTextFormatter(s string) string {
	// replace & < >
	s = strings.Replace(s, "&", "&amp;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	s = strings.Replace(s, ">", "&gt;", -1)
	return s
}

// SlackShortTextFormatter replaces &, <, > with HTML characters
func SlackShortTextFormatter(s string) string {
	s = strings.Split(s, "\n")[0]
	// replace & < >
	s = strings.Replace(s, "&", "&amp;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	s = strings.Replace(s, ">", "&gt;", -1)
	return s
}

// SlackLinkFormatter creates a link compatible with slack
func SlackLinkFormatter(url string, text string) string {
	return fmt.Sprintf("<%s|%s>", url, SlackTextFormatter(text))
}

func getSlackCreatePayload(p *api.CreatePayload, slack *SlackMeta) (*SlackPayload, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)

	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.Name)
	refLink := SlackLinkFormatter(p.Repo.HTMLURL+"/src/"+refName, refName)
	text := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, p.Sender.UserName)

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}, nil
}

func getSlackPushPayload(p *api.PushPayload, slack *SlackMeta) (*SlackPayload, error) {
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
	text := fmt.Sprintf("[%s:%s] %s pushed by %s", repoLink, branchLink, commitString, p.Pusher.UserName)

	var attachmentText string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		attachmentText += fmt.Sprintf("%s: %s - %s", SlackLinkFormatter(commit.URL, commit.ID[:7]), SlackShortTextFormatter(commit.Message), SlackTextFormatter(commit.Author.Name))
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			attachmentText += "\n"
		}
	}

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
		Attachments: []SlackAttachment{{
			Color: slack.Color,
			Text:  attachmentText,
		}},
	}, nil
}

func getSlackPullRequestPayload(p *api.PullRequestPayload, slack *SlackMeta) (*SlackPayload, error) {
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	titleLink := SlackLinkFormatter(fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index),
		fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title))
	var text, title, attachmentText string
	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Pull request submitted by %s", p.Repository.FullName, senderLink)
		title = titleLink
		attachmentText = SlackTextFormatter(p.PullRequest.Body)
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
		attachmentText = SlackTextFormatter(p.PullRequest.Body)
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

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
		Attachments: []SlackAttachment{{
			Color: slack.Color,
			Title: title,
			Text:  attachmentText,
		}},
	}, nil
}

func getSlackRepositoryPayload(p *api.RepositoryPayload, slack *SlackMeta) (*SlackPayload, error) {
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	var text, title, attachmentText string
	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", p.Repository.FullName, senderLink)
		title = p.Repository.HTMLURL
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", p.Repository.FullName, senderLink)
	}

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
		Attachments: []SlackAttachment{{
			Color: slack.Color,
			Title: title,
			Text:  attachmentText,
		}},
	}, nil
}

// GetSlackPayload converts a slack webhook into a SlackPayload
func GetSlackPayload(p api.Payloader, event HookEventType, meta string) (*SlackPayload, error) {
	s := new(SlackPayload)

	slack := &SlackMeta{}
	if err := json.Unmarshal([]byte(meta), &slack); err != nil {
		return s, errors.New("GetSlackPayload meta json:" + err.Error())
	}

	switch event {
	case HookEventCreate:
		return getSlackCreatePayload(p.(*api.CreatePayload), slack)
	case HookEventPush:
		return getSlackPushPayload(p.(*api.PushPayload), slack)
	case HookEventPullRequest:
		return getSlackPullRequestPayload(p.(*api.PullRequestPayload), slack)
	case HookEventRepository:
		return getSlackRepositoryPayload(p.(*api.RepositoryPayload), slack)
	}

	return s, nil
}
