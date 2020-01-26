// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

// SlackMeta contains the slack metadata
type SlackMeta struct {
	Channel  string `json:"channel"`
	Username string `json:"username"`
	IconURL  string `json:"icon_url"`
	Color    string `json:"color"`
}

// GetSlackHook returns slack metadata
func GetSlackHook(w *models.Webhook) *SlackMeta {
	s := &SlackMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetSlackHook(%d): %v", w.ID, err)
	}
	return s
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
	Fallback  string `json:"fallback"`
	Color     string `json:"color"`
	Title     string `json:"title"`
	TitleLink string `json:"title_link"`
	Text      string `json:"text"`
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

// SlackLinkToRef slack-formatter link to a repo ref
func SlackLinkToRef(repoURL, ref string) string {
	refName := git.RefEndName(ref)
	switch {
	case strings.HasPrefix(ref, git.BranchPrefix):
		return SlackLinkFormatter(repoURL+"/src/branch/"+refName, refName)
	case strings.HasPrefix(ref, git.TagPrefix):
		return SlackLinkFormatter(repoURL+"/src/tag/"+refName, refName)
	default:
		return SlackLinkFormatter(repoURL+"/src/commit/"+refName, refName)
	}
}

func getSlackCreatePayload(p *api.CreatePayload, slack *SlackMeta) (*SlackPayload, error) {
	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	refLink := SlackLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, p.Sender.UserName)

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}, nil
}

// getSlackDeletePayload composes Slack payload for delete a branch or tag.
func getSlackDeletePayload(p *api.DeletePayload, slack *SlackMeta) (*SlackPayload, error) {
	refName := git.RefEndName(p.Ref)
	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("[%s:%s] %s deleted by %s", repoLink, refName, p.RefType, p.Sender.UserName)
	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}, nil
}

// getSlackForkPayload composes Slack payload for forked by a repository.
func getSlackForkPayload(p *api.ForkPayload, slack *SlackMeta) (*SlackPayload, error) {
	baseLink := SlackLinkFormatter(p.Forkee.HTMLURL, p.Forkee.FullName)
	forkLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("%s is forked to %s", baseLink, forkLink)
	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}, nil
}

func getSlackIssuesPayload(p *api.IssuePayload, slack *SlackMeta) (*SlackPayload, error) {
	text, issueTitle, attachmentText, color := getIssuesPayloadInfo(p, SlackLinkFormatter, true)

	pl := &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}
	if attachmentText != "" {
		attachmentText = SlackTextFormatter(attachmentText)
		issueTitle = SlackTextFormatter(issueTitle)
		pl.Attachments = []SlackAttachment{{
			Color:     fmt.Sprintf("%x", color),
			Title:     issueTitle,
			TitleLink: p.Issue.HTMLURL,
			Text:      attachmentText,
		}}
	}

	return pl, nil
}

func getSlackIssueCommentPayload(p *api.IssueCommentPayload, slack *SlackMeta) (*SlackPayload, error) {
	text, issueTitle, color := getIssueCommentPayloadInfo(p, SlackLinkFormatter, true)

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
		Attachments: []SlackAttachment{{
			Color:     fmt.Sprintf("%x", color),
			Title:     issueTitle,
			TitleLink: p.Comment.HTMLURL,
			Text:      SlackTextFormatter(p.Comment.Body),
		}},
	}, nil
}

func getSlackReleasePayload(p *api.ReleasePayload, slack *SlackMeta) (*SlackPayload, error) {
	text, _ := getReleasePayloadInfo(p, SlackLinkFormatter, true)

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

	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	branchLink := SlackLinkToRef(p.Repo.HTMLURL, p.Ref)
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
			Color:     slack.Color,
			Title:     p.Repo.HTMLURL,
			TitleLink: p.Repo.HTMLURL,
			Text:      attachmentText,
		}},
	}, nil
}

func getSlackPullRequestPayload(p *api.PullRequestPayload, slack *SlackMeta) (*SlackPayload, error) {
	text, issueTitle, attachmentText, color := getPullRequestPayloadInfo(p, SlackLinkFormatter, true)

	pl := &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}
	if attachmentText != "" {
		attachmentText = SlackTextFormatter(p.PullRequest.Body)
		issueTitle = SlackTextFormatter(issueTitle)
		pl.Attachments = []SlackAttachment{{
			Color:     fmt.Sprintf("%x", color),
			Title:     issueTitle,
			TitleLink: p.PullRequest.URL,
			Text:      attachmentText,
		}}
	}

	return pl, nil
}

func getSlackPullRequestApprovalPayload(p *api.PullRequestPayload, slack *SlackMeta, event models.HookEventType) (*SlackPayload, error) {
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	title := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index)
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: [%s](%s) by %s", repoLink, action, title, titleLink, senderLink)
	}

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}, nil
}

func getSlackRepositoryPayload(p *api.RepositoryPayload, slack *SlackMeta) (*SlackPayload, error) {
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", repoLink, senderLink)
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", repoLink, senderLink)
	}

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}, nil
}

// GetSlackPayload converts a slack webhook into a SlackPayload
func GetSlackPayload(p api.Payloader, event models.HookEventType, meta string) (*SlackPayload, error) {
	s := new(SlackPayload)

	slack := &SlackMeta{}
	if err := json.Unmarshal([]byte(meta), &slack); err != nil {
		return s, errors.New("GetSlackPayload meta json:" + err.Error())
	}

	switch event {
	case models.HookEventCreate:
		return getSlackCreatePayload(p.(*api.CreatePayload), slack)
	case models.HookEventDelete:
		return getSlackDeletePayload(p.(*api.DeletePayload), slack)
	case models.HookEventFork:
		return getSlackForkPayload(p.(*api.ForkPayload), slack)
	case models.HookEventIssues:
		return getSlackIssuesPayload(p.(*api.IssuePayload), slack)
	case models.HookEventIssueComment:
		return getSlackIssueCommentPayload(p.(*api.IssueCommentPayload), slack)
	case models.HookEventPush:
		return getSlackPushPayload(p.(*api.PushPayload), slack)
	case models.HookEventPullRequest:
		return getSlackPullRequestPayload(p.(*api.PullRequestPayload), slack)
	case models.HookEventPullRequestRejected, models.HookEventPullRequestApproved, models.HookEventPullRequestComment:
		return getSlackPullRequestApprovalPayload(p.(*api.PullRequestPayload), slack, event)
	case models.HookEventRepository:
		return getSlackRepositoryPayload(p.(*api.RepositoryPayload), slack)
	case models.HookEventRelease:
		return getSlackReleasePayload(p.(*api.ReleasePayload), slack)
	}

	return s, nil
}
