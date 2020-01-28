// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
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
	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.Name)
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
	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.Name)
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
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	title := SlackTextFormatter(fmt.Sprintf("#%d %s", p.Index, p.Issue.Title))
	titleLink := fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Index)
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text, attachmentText string

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Issue opened by %s", repoLink, senderLink)
		attachmentText = SlackTextFormatter(p.Issue.Body)
	case api.HookIssueClosed:
		text = fmt.Sprintf("[%s] Issue closed: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Issue re-opened: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Issue edited: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
		attachmentText = SlackTextFormatter(p.Issue.Body)
	case api.HookIssueAssigned:
		text = fmt.Sprintf("[%s] Issue assigned to %s: [%s](%s) by %s", repoLink,
			SlackLinkFormatter(setting.AppURL+p.Issue.Assignee.UserName, p.Issue.Assignee.UserName),
			title, titleLink, senderLink)
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Issue unassigned: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Issue labels updated: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Issue labels cleared: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Issue synchronized: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.Issue.Milestone.ID)
		text = fmt.Sprintf("[%s] Issue milestoned to [%s](%s): [%s](%s) by %s", repoLink,
			p.Issue.Milestone.Title, mileStoneLink, title, titleLink, senderLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Issue milestone cleared: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	}

	pl := &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}
	if attachmentText != "" {
		pl.Attachments = []SlackAttachment{{
			Color:     slack.Color,
			Title:     title,
			TitleLink: titleLink,
			Text:      attachmentText,
		}}
	}

	return pl, nil
}

func getSlackIssueCommentPayload(p *api.IssueCommentPayload, slack *SlackMeta) (*SlackPayload, error) {
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	title := SlackTextFormatter(fmt.Sprintf("#%d %s", p.Issue.Index, p.Issue.Title))
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text, titleLink, attachmentText string

	switch p.Action {
	case api.HookIssueCommentCreated:
		text = fmt.Sprintf("[%s] New comment created by %s", repoLink, senderLink)
		titleLink = fmt.Sprintf("%s/issues/%d#%s", p.Repository.HTMLURL, p.Issue.Index, CommentHashTag(p.Comment.ID))
		attachmentText = SlackTextFormatter(p.Comment.Body)
	case api.HookIssueCommentEdited:
		text = fmt.Sprintf("[%s] Comment edited by %s", repoLink, senderLink)
		titleLink = fmt.Sprintf("%s/issues/%d#%s", p.Repository.HTMLURL, p.Issue.Index, CommentHashTag(p.Comment.ID))
		attachmentText = SlackTextFormatter(p.Comment.Body)
	case api.HookIssueCommentDeleted:
		text = fmt.Sprintf("[%s] Comment deleted by %s", repoLink, senderLink)
		titleLink = fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Issue.Index)
		attachmentText = SlackTextFormatter(p.Comment.Body)
	}

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
		Attachments: []SlackAttachment{{
			Color:     slack.Color,
			Title:     title,
			TitleLink: titleLink,
			Text:      attachmentText,
		}},
	}, nil
}

func getSlackReleasePayload(p *api.ReleasePayload, slack *SlackMeta) (*SlackPayload, error) {
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.Name)
	refLink := SlackLinkFormatter(p.Repository.HTMLURL+"/src/"+p.Release.TagName, p.Release.TagName)
	var text string

	switch p.Action {
	case api.HookReleasePublished:
		text = fmt.Sprintf("[%s] new release %s published by %s", repoLink, refLink, p.Sender.UserName)
	case api.HookReleaseUpdated:
		text = fmt.Sprintf("[%s] new release %s updated by %s", repoLink, refLink, p.Sender.UserName)
	case api.HookReleaseDeleted:
		text = fmt.Sprintf("[%s] new release %s deleted by %s", repoLink, refLink, p.Sender.UserName)
	}

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

	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.Name)
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
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	title := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index)
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text, attachmentText string

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Pull request opened by %s", repoLink, senderLink)
		attachmentText = SlackTextFormatter(p.PullRequest.Body)
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			text = fmt.Sprintf("[%s] Pull request merged: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
		} else {
			text = fmt.Sprintf("[%s] Pull request closed: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
		}
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Pull request re-opened: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Pull request edited: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
		attachmentText = SlackTextFormatter(p.PullRequest.Body)
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = SlackLinkFormatter(setting.AppURL+user.UserName, user.UserName)
		}
		text = fmt.Sprintf("[%s] Pull request assigned to %s: [%s](%s) by %s", repoLink,
			strings.Join(list, ", "),
			title, titleLink, senderLink)
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Pull request unassigned: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Pull request labels updated: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Pull request labels cleared: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Pull request synchronized: [%s](%s) by %s", repoLink, title, titleLink, senderLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.PullRequest.Milestone.ID)
		text = fmt.Sprintf("[%s] Pull request milestoned to [%s](%s): [%s](%s) %s", repoLink,
			p.PullRequest.Milestone.Title, mileStoneLink, title, titleLink, senderLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Pull request milestone cleared: [%s](%s) %s", repoLink, title, titleLink, senderLink)
	}

	pl := &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
	}
	if attachmentText != "" {
		pl.Attachments = []SlackAttachment{{
			Color:     slack.Color,
			Title:     title,
			TitleLink: titleLink,
			Text:      attachmentText,
		}}
	}

	return pl, nil
}

func getSlackPullRequestApprovalPayload(p *api.PullRequestPayload, slack *SlackMeta, event HookEventType) (*SlackPayload, error) {
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
	var text string

	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", p.Repository.FullName, senderLink)
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", p.Repository.FullName, senderLink)
	}

	return &SlackPayload{
		Channel:  slack.Channel,
		Text:     text,
		Username: slack.Username,
		IconURL:  slack.IconURL,
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
	case HookEventDelete:
		return getSlackDeletePayload(p.(*api.DeletePayload), slack)
	case HookEventFork:
		return getSlackForkPayload(p.(*api.ForkPayload), slack)
	case HookEventIssues:
		return getSlackIssuesPayload(p.(*api.IssuePayload), slack)
	case HookEventIssueComment:
		return getSlackIssueCommentPayload(p.(*api.IssueCommentPayload), slack)
	case HookEventPush:
		return getSlackPushPayload(p.(*api.PushPayload), slack)
	case HookEventPullRequest:
		return getSlackPullRequestPayload(p.(*api.PullRequestPayload), slack)
	case HookEventPullRequestRejected, HookEventPullRequestApproved, HookEventPullRequestComment:
		return getSlackPullRequestApprovalPayload(p.(*api.PullRequestPayload), slack, event)
	case HookEventRepository:
		return getSlackRepositoryPayload(p.(*api.RepositoryPayload), slack)
	case HookEventRelease:
		return getSlackReleasePayload(p.(*api.ReleasePayload), slack)
	}

	return s, nil
}
