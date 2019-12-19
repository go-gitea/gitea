// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

type linkFormatter = func(string, string) string

// noneLinkFormatter does not create a link but just returns the text
func noneLinkFormatter(url string, text string) string {
	return text
}

// htmlLinkFormatter creates a HTML link
func htmlLinkFormatter(url string, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, url, text)
}

func getIssuesPayloadInfo(p *api.IssuePayload, linkFormatter linkFormatter) (string, string, string, int) {
	senderLink := linkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	issueTitle := fmt.Sprintf("#%d %s", p.Index, p.Issue.Title)
	titleLink := linkFormatter(p.Issue.URL, issueTitle)
	var text string
	color := yellowColor

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Issue opened by %s", repoLink, senderLink)
		color = orangeColor
	case api.HookIssueClosed:
		text = fmt.Sprintf("[%s] Issue closed: %s by %s", repoLink, titleLink, senderLink)
		color = redColor
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Issue re-opened: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Issue edited: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueAssigned:
		text = fmt.Sprintf("[%s] Issue assigned to %s: %s by %s", repoLink,
			linkFormatter(setting.AppURL+p.Issue.Assignee.UserName, p.Issue.Assignee.UserName),
			titleLink, senderLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Issue unassigned: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Issue labels updated: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Issue labels cleared: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Issue synchronized: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.Issue.Milestone.ID)
		text = fmt.Sprintf("[%s] Issue milestoned to %s: %s by %s", repoLink,
			linkFormatter(mileStoneLink, p.Issue.Milestone.Title), titleLink, senderLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Issue milestone cleared: %s by %s", repoLink, titleLink, senderLink)
	}

	var attachmentText string
	if p.Action == api.HookIssueOpened || p.Action == api.HookIssueEdited {
		attachmentText = p.Issue.Body
	}

	return text, issueTitle, attachmentText, color
}

func getPullRequestPayloadInfo(p *api.PullRequestPayload, linkFormatter linkFormatter) (string, string, string, int) {
	senderLink := linkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	issueTitle := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := linkFormatter(p.PullRequest.URL, issueTitle)
	var text string
	color := yellowColor

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Pull request opened by %s", repoLink, senderLink)
		color = greenColor
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			text = fmt.Sprintf("[%s] Pull request merged: %s by %s", repoLink, titleLink, senderLink)
			color = purpleColor
		} else {
			text = fmt.Sprintf("[%s] Pull request closed: %s by %s", repoLink, titleLink, senderLink)
			color = redColor
		}
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Pull request re-opened: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Pull request edited: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = linkFormatter(setting.AppURL+user.UserName, user.UserName)
		}
		text = fmt.Sprintf("[%s] Pull request assigned to %s: %s by %s", repoLink,
			strings.Join(list, ", "),
			titleLink, senderLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Pull request unassigned: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Pull request labels updated: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Pull request labels cleared: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Pull request synchronized: %s by %s", repoLink, titleLink, senderLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.PullRequest.Milestone.ID)
		text = fmt.Sprintf("[%s] Pull request milestoned to %s: %s by %s", repoLink,
			linkFormatter(mileStoneLink, p.PullRequest.Milestone.Title), titleLink, senderLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Pull request milestone cleared: %s %s", repoLink, titleLink, senderLink)
	}

	var attachmentText string
	if p.Action == api.HookIssueOpened || p.Action == api.HookIssueEdited {
		attachmentText = p.PullRequest.Body
	}

	return text, issueTitle, attachmentText, color
}
