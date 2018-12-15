// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"code.gitea.io/git"
	api "code.gitea.io/sdk/gitea"
)

// TypetalkPayload contains information for posting messages on Typetalk
type TypetalkPayload struct {
	Message string `json:"message"`
}

// SetSecret sets the Typetalk secret
func (p *TypetalkPayload) SetSecret(_ string) {}

// JSONPayload Marshals TypetalkPayload to json
func (p *TypetalkPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func formatRepositoryLinkForTypetalk(name, url string) string {
	return fmt.Sprintf("[%s](%s)", name, url)
}

func formatIssueLinkForTypetalk(issueTitle, repositoryURL string, issueNumber int64) string {
	return fmt.Sprintf("[#%d %s](%s/issues/%d)", issueNumber, issueTitle, repositoryURL, issueNumber)
}

func formatIssueCommentLinkForTypetalk(repositoryURL string, issueID, commentID int64) string {
	return fmt.Sprintf("[#%d](%s/issues/%d#%s)", commentID, repositoryURL, issueID, CommentHashTag(commentID))
}

func formatPullRequestLinkForTypetalk(pullRequestTitle, pullRequestyURL string, pullRequestID int64) string {
	return fmt.Sprintf("[#%d %s](%s)", pullRequestID, pullRequestTitle, pullRequestyURL)
}

func formatTagLinkForTypetalk(name, url, tag string) string {
	return fmt.Sprintf("[%s:%s](%s/src/tag/%s)", name, tag, url, tag)
}

func getTypetalkCreatePayload(p *api.CreatePayload) (*TypetalkPayload, error) {
	repoLink := formatRepositoryLinkForTypetalk(p.Repo.FullName, p.Repo.HTMLURL)
	refName := git.RefEndName(p.Ref)
	message := fmt.Sprintf("[%s] %s %s created", repoLink, p.RefType, refName)

	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkDeletePayload(p *api.DeletePayload) (*TypetalkPayload, error) {
	repoLink := formatRepositoryLinkForTypetalk(p.Repo.FullName, p.Repo.HTMLURL)
	refName := git.RefEndName(p.Ref)
	message := fmt.Sprintf("[%s] %s %s deleted", repoLink, p.RefType, refName)

	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkForkPayload(p *api.ForkPayload) (*TypetalkPayload, error) {
	origin := fmt.Sprintf("[%s](%s)", p.Forkee.FullName, p.Forkee.HTMLURL)
	forked := fmt.Sprintf("[%s](%s)", p.Repo.FullName, p.Repo.HTMLURL)
	message := fmt.Sprintf("%s is forked to %s", origin, forked)

	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkIssuesPayload(p *api.IssuePayload) (*TypetalkPayload, error) {

	repoLink := formatRepositoryLinkForTypetalk(p.Repository.FullName, p.Repository.HTMLURL)
	issueLink := formatIssueLinkForTypetalk(p.Issue.Title, p.Repository.HTMLURL, p.Index)

	var title, text string
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] %s Issue opened by %s", repoLink, issueLink, p.Sender.UserName)
		text = p.Issue.Body
	case api.HookIssueClosed:
		title = fmt.Sprintf("[%s] %s Issue closed by %s", repoLink, issueLink, p.Sender.UserName)
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] %s Issue re-opened by %s", repoLink, issueLink, p.Sender.UserName)
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] %s Issue edited by %s", repoLink, issueLink, p.Sender.UserName)
		text = p.Issue.Body
	case api.HookIssueAssigned:
		title = fmt.Sprintf("[%s] %s Issue assigned to %s", repoLink, issueLink, p.Issue.Assignee.UserName)
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] %s Issue unassigned by %s", repoLink, issueLink, p.Sender.UserName)
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] %s Issue labels updated by %s", repoLink, issueLink, p.Sender.UserName)
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] %s Issue labels cleared by %s", repoLink, issueLink, p.Sender.UserName)
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] %s Issue synchronized by %s", repoLink, issueLink, p.Sender.UserName)
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] %s Issue milestones updated by %s", repoLink, issueLink, p.Sender.UserName)
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] %s Issue milestone cleared by %s", repoLink, issueLink, p.Sender.UserName)
	}
	message := fmt.Sprintf("%s\n%s", title, text)

	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkIssueCommentPayload(p *api.IssueCommentPayload) (*TypetalkPayload, error) {

	repoLink := formatRepositoryLinkForTypetalk(p.Repository.FullName, p.Repository.HTMLURL)
	issueLink := formatIssueLinkForTypetalk(p.Issue.Title, p.Repository.HTMLURL, p.Issue.Index)
	issueCommentLink := formatIssueCommentLinkForTypetalk(p.Repository.HTMLURL, p.Issue.Index, p.Comment.ID)

	var title, text string
	switch p.Action {
	case api.HookIssueCommentCreated:
		title = fmt.Sprintf("[%s] %s New comment %s created by %s ", repoLink, issueLink, issueCommentLink, p.Sender.UserName)
		text = p.Comment.Body
	case api.HookIssueCommentEdited:
		title = fmt.Sprintf("[%s] %s Comment %s edited by %s", repoLink, issueLink, issueCommentLink, p.Sender.UserName)
		text = p.Comment.Body
	case api.HookIssueCommentDeleted:
		title = fmt.Sprintf("[%s] %s Comment #%d deleted by %s", repoLink, issueLink, p.Comment.ID, p.Sender.UserName)
	}

	message := fmt.Sprintf("%s\n%s", title, text)
	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkPushPayload(p *api.PushPayload) (*TypetalkPayload, error) {

	branchName := git.RefEndName(p.Ref)

	var titleLink, commitDesc string
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

	title := fmt.Sprintf("[[%s:%s] %s](%s)", p.Repo.FullName, branchName, commitDesc, titleLink)

	var text string
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = " - " + commit.Author.Name
		}
		t := fmt.Sprintf("[%s](%s) %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n")) + authorName

		// Typetalk accepts message shorter than 4000 characters.
		if utf8.RuneCountInString(text)+utf8.RuneCountInString(t) < 4000 {
			text += t
			// Add linebreak to each commit but the last
			if i < len(p.Commits)-1 {
				text += "\n"
			}
		} else {
			text += "..."
			break
		}
	}

	message := fmt.Sprintf("%s\n%s", title, text)

	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkPullRequestPayload(p *api.PullRequestPayload) (*TypetalkPayload, error) {

	repoLink := formatRepositoryLinkForTypetalk(p.Repository.FullName, p.Repository.HTMLURL)
	pullRequestLink := formatPullRequestLinkForTypetalk(p.PullRequest.Title, p.PullRequest.HTMLURL, p.Index)

	var text, title string
	switch p.Action {
	case api.HookIssueOpened:
		title = fmt.Sprintf("[%s] %s Pull request opened by %s", repoLink, pullRequestLink, p.Sender.UserName)
		text = p.PullRequest.Body
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			title = fmt.Sprintf("[%s] %s Pull request merged by %s", repoLink, pullRequestLink, p.Sender.UserName)
		} else {
			title = fmt.Sprintf("[%s] %s Pull request closed by %s", repoLink, pullRequestLink, p.Sender.UserName)
		}
	case api.HookIssueReOpened:
		title = fmt.Sprintf("[%s] %s Pull request re-opened by %s", repoLink, pullRequestLink, p.Sender.UserName)
	case api.HookIssueEdited:
		title = fmt.Sprintf("[%s] %s Pull request edited by %s", repoLink, pullRequestLink, p.Sender.UserName)
		text = p.PullRequest.Body
	case api.HookIssueAssigned:
		list, err := MakeAssigneeList(&Issue{ID: p.PullRequest.ID})
		if err != nil {
			return &TypetalkPayload{}, err
		}
		title = fmt.Sprintf("[%s] %s Pull request assigned to %s", repoLink, pullRequestLink, list)
	case api.HookIssueUnassigned:
		title = fmt.Sprintf("[%s] %s Pull request unassigned by %s", repoLink, pullRequestLink, p.Sender.UserName)
	case api.HookIssueLabelUpdated:
		title = fmt.Sprintf("[%s] %s Pull request labels updated by %s", repoLink, pullRequestLink, p.Sender.UserName)
	case api.HookIssueLabelCleared:
		title = fmt.Sprintf("[%s] %s Pull request labels cleared by %s", repoLink, pullRequestLink, p.Sender.UserName)
	case api.HookIssueSynchronized:
		title = fmt.Sprintf("[%s] %s Pull request synchronized by %s", repoLink, pullRequestLink, p.Sender.UserName)
	case api.HookIssueMilestoned:
		title = fmt.Sprintf("[%s] %s Pull request milestones updated by %s", repoLink, pullRequestLink, p.Sender.UserName)
	case api.HookIssueDemilestoned:
		title = fmt.Sprintf("[%s] %s Pull request milestones cleared by %s", repoLink, pullRequestLink, p.Sender.UserName)
	}

	message := fmt.Sprintf("%s\n%s", title, text)

	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkRepositoryPayload(p *api.RepositoryPayload) (*TypetalkPayload, error) {

	var message string
	switch p.Action {
	case api.HookRepoCreated:
		message = fmt.Sprintf("[%s] Repository created", formatRepositoryLinkForTypetalk(p.Repository.FullName, p.Repository.HTMLURL))
	case api.HookRepoDeleted:
		message = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
	}

	return &TypetalkPayload{
		Message: message,
	}, nil
}

func getTypetalkReleasePayload(p *api.ReleasePayload) (*TypetalkPayload, error) {

	tagLink := formatTagLinkForTypetalk(p.Repository.FullName, p.Repository.HTMLURL, p.Release.TagName)

	var message string
	switch p.Action {
	case api.HookReleasePublished:
		message = fmt.Sprintf("[%s] Release created", tagLink)
	case api.HookReleaseUpdated:
		message = fmt.Sprintf("[%s] Release updated", tagLink)
	case api.HookReleaseDeleted:
		message = fmt.Sprintf("[%s:%s] Release deleted", p.Repository.FullName, p.Release.TagName)
	}

	return &TypetalkPayload{
		Message: message,
	}, nil
}

// GetTypetalkPayload converts a Typetalk webhook into a TypetalkPayload
func GetTypetalkPayload(p api.Payloader, event HookEventType, meta string) (*TypetalkPayload, error) {
	s := new(TypetalkPayload)

	switch event {
	case HookEventCreate:
		return getTypetalkCreatePayload(p.(*api.CreatePayload))
	case HookEventDelete:
		return getTypetalkDeletePayload(p.(*api.DeletePayload))
	case HookEventFork:
		return getTypetalkForkPayload(p.(*api.ForkPayload))
	case HookEventIssues:
		return getTypetalkIssuesPayload(p.(*api.IssuePayload))
	case HookEventIssueComment:
		return getTypetalkIssueCommentPayload(p.(*api.IssueCommentPayload))
	case HookEventPush:
		return getTypetalkPushPayload(p.(*api.PushPayload))
	case HookEventPullRequest:
		return getTypetalkPullRequestPayload(p.(*api.PullRequestPayload))
	case HookEventRepository:
		return getTypetalkRepositoryPayload(p.(*api.RepositoryPayload))
	case HookEventRelease:
		return getTypetalkReleasePayload(p.(*api.ReleasePayload))
	}

	return s, nil
}
