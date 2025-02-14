// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

type linkFormatter = func(string, string) string

// noneLinkFormatter does not create a link but just returns the text
func noneLinkFormatter(url, text string) string {
	return text
}

// htmlLinkFormatter creates a HTML link
func htmlLinkFormatter(url, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(url), html.EscapeString(text))
}

// getPullRequestInfo gets the information for a pull request
func getPullRequestInfo(p *api.PullRequestPayload) (title, link, by, operator, operateResult, assignees string) {
	title = fmt.Sprintf("[PullRequest-%s #%d]: %s\n%s", p.Repository.FullName, p.PullRequest.Index, p.Action, p.PullRequest.Title)
	assignList := p.PullRequest.Assignees
	assignStringList := make([]string, len(assignList))

	for i, user := range assignList {
		assignStringList[i] = user.UserName
	}
	if p.Action == api.HookIssueAssigned {
		operateResult = fmt.Sprintf("%s assign this to %s", p.Sender.UserName, assignList[len(assignList)-1].UserName)
	} else if p.Action == api.HookIssueUnassigned {
		operateResult = fmt.Sprintf("%s unassigned this for someone", p.Sender.UserName)
	} else if p.Action == api.HookIssueMilestoned {
		operateResult = fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.PullRequest.Milestone.ID)
	}
	link = p.PullRequest.HTMLURL
	by = fmt.Sprintf("PullRequest by %s", p.PullRequest.Poster.UserName)
	if len(assignStringList) > 0 {
		assignees = fmt.Sprintf("Assignees: %s", strings.Join(assignStringList, ", "))
	}
	operator = fmt.Sprintf("Operator: %s", p.Sender.UserName)
	return title, link, by, operator, operateResult, assignees
}

// getIssuesInfo gets the information for an issue
func getIssuesInfo(p *api.IssuePayload) (issueTitle, link, by, operator, operateResult, assignees string) {
	issueTitle = fmt.Sprintf("[Issue-%s #%d]: %s\n%s", p.Repository.FullName, p.Issue.Index, p.Action, p.Issue.Title)
	assignList := p.Issue.Assignees
	assignStringList := make([]string, len(assignList))

	for i, user := range assignList {
		assignStringList[i] = user.UserName
	}
	if p.Action == api.HookIssueAssigned {
		operateResult = fmt.Sprintf("%s assign this to %s", p.Sender.UserName, assignList[len(assignList)-1].UserName)
	} else if p.Action == api.HookIssueUnassigned {
		operateResult = fmt.Sprintf("%s unassigned this for someone", p.Sender.UserName)
	} else if p.Action == api.HookIssueMilestoned {
		operateResult = fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.Issue.Milestone.ID)
	}
	link = p.Issue.HTMLURL
	by = fmt.Sprintf("Issue by %s", p.Issue.Poster.UserName)
	if len(assignStringList) > 0 {
		assignees = fmt.Sprintf("Assignees: %s", strings.Join(assignStringList, ", "))
	}
	operator = fmt.Sprintf("Operator: %s", p.Sender.UserName)
	return issueTitle, link, by, operator, operateResult, assignees
}

// getIssuesCommentInfo gets the information for a comment
func getIssuesCommentInfo(p *api.IssueCommentPayload) (title, link, by, operator string) {
	title = fmt.Sprintf("[Comment-%s #%d]: %s\n%s", p.Repository.FullName, p.Issue.Index, p.Action, p.Issue.Title)
	link = p.Issue.HTMLURL
	if p.IsPull {
		by = fmt.Sprintf("PullRequest by %s", p.Issue.Poster.UserName)
	} else {
		by = fmt.Sprintf("Issue by %s", p.Issue.Poster.UserName)
	}
	operator = fmt.Sprintf("Operator: %s", p.Sender.UserName)
	return title, link, by, operator
}

func getIssuesPayloadInfo(p *api.IssuePayload, linkFormatter linkFormatter, withSender bool) (text, issueTitle, extraMarkdown string, color int) {
	color = yellowColor
	issueTitle = fmt.Sprintf("#%d %s", p.Index, p.Issue.Title)
	titleLink := linkFormatter(fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Index), issueTitle)
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Issue opened: %s", repoLink, titleLink)
		color = orangeColor
	case api.HookIssueClosed:
		text = fmt.Sprintf("[%s] Issue closed: %s", repoLink, titleLink)
		color = redColor
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Issue re-opened: %s", repoLink, titleLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Issue edited: %s", repoLink, titleLink)
	case api.HookIssueAssigned:
		list := make([]string, len(p.Issue.Assignees))
		for i, user := range p.Issue.Assignees {
			list[i] = linkFormatter(setting.AppURL+url.PathEscape(user.UserName), user.UserName)
		}
		text = fmt.Sprintf("[%s] Issue assigned to %s: %s", repoLink, strings.Join(list, ", "), titleLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Issue unassigned: %s", repoLink, titleLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Issue labels updated: %s", repoLink, titleLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Issue labels cleared: %s", repoLink, titleLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Issue synchronized: %s", repoLink, titleLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.Issue.Milestone.ID)
		text = fmt.Sprintf("[%s] Issue milestoned to %s: %s", repoLink,
			linkFormatter(mileStoneLink, p.Issue.Milestone.Title), titleLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Issue milestone cleared: %s", repoLink, titleLink)
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName))
	}

	if p.Action == api.HookIssueOpened || p.Action == api.HookIssueEdited {
		extraMarkdown = p.Issue.Body
	}

	return text, issueTitle, extraMarkdown, color
}

func getPullRequestPayloadInfo(p *api.PullRequestPayload, linkFormatter linkFormatter, withSender bool) (text, issueTitle, extraMarkdown string, color int) {
	color = yellowColor
	issueTitle = fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := linkFormatter(p.PullRequest.URL, issueTitle)
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Pull request opened: %s", repoLink, titleLink)
		extraMarkdown = p.PullRequest.Body
		color = greenColor
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			text = fmt.Sprintf("[%s] Pull request merged: %s", repoLink, titleLink)
			color = purpleColor
		} else {
			text = fmt.Sprintf("[%s] Pull request closed: %s", repoLink, titleLink)
			color = redColor
		}
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Pull request re-opened: %s", repoLink, titleLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Pull request edited: %s", repoLink, titleLink)
		extraMarkdown = p.PullRequest.Body
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = linkFormatter(setting.AppURL+user.UserName, user.UserName)
		}
		text = fmt.Sprintf("[%s] Pull request assigned to %s: %s", repoLink,
			strings.Join(list, ", "), titleLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Pull request unassigned: %s", repoLink, titleLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Pull request labels updated: %s", repoLink, titleLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Pull request labels cleared: %s", repoLink, titleLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Pull request synchronized: %s", repoLink, titleLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.PullRequest.Milestone.ID)
		text = fmt.Sprintf("[%s] Pull request milestoned to %s: %s", repoLink,
			linkFormatter(mileStoneLink, p.PullRequest.Milestone.Title), titleLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Pull request milestone cleared: %s", repoLink, titleLink)
	case api.HookIssueReviewed:
		text = fmt.Sprintf("[%s] Pull request reviewed: %s", repoLink, titleLink)
		extraMarkdown = p.Review.Content
	case api.HookIssueReviewRequested:
		text = fmt.Sprintf("[%s] Pull request review requested: %s", repoLink, titleLink)
	case api.HookIssueReviewRequestRemoved:
		text = fmt.Sprintf("[%s] Pull request review request removed: %s", repoLink, titleLink)
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName))
	}

	return text, issueTitle, extraMarkdown, color
}

func getReleasePayloadInfo(p *api.ReleasePayload, linkFormatter linkFormatter, withSender bool) (text string, color int) {
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	refLink := linkFormatter(p.Repository.HTMLURL+"/releases/tag/"+util.PathEscapeSegments(p.Release.TagName), p.Release.TagName)

	switch p.Action {
	case api.HookReleasePublished:
		text = fmt.Sprintf("[%s] Release created: %s", repoLink, refLink)
		color = greenColor
	case api.HookReleaseUpdated:
		text = fmt.Sprintf("[%s] Release updated: %s", repoLink, refLink)
		color = yellowColor
	case api.HookReleaseDeleted:
		text = fmt.Sprintf("[%s] Release deleted: %s", repoLink, refLink)
		color = redColor
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName))
	}

	return text, color
}

func getWikiPayloadInfo(p *api.WikiPayload, linkFormatter linkFormatter, withSender bool) (string, int, string) {
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	pageLink := linkFormatter(p.Repository.HTMLURL+"/wiki/"+url.PathEscape(p.Page), p.Page)

	var text string
	color := greenColor

	switch p.Action {
	case api.HookWikiCreated:
		text = fmt.Sprintf("[%s] New wiki page '%s'", repoLink, pageLink)
	case api.HookWikiEdited:
		text = fmt.Sprintf("[%s] Wiki page '%s' edited", repoLink, pageLink)
		color = yellowColor
	case api.HookWikiDeleted:
		text = fmt.Sprintf("[%s] Wiki page '%s' deleted", repoLink, pageLink)
		color = redColor
	}

	if p.Action != api.HookWikiDeleted && p.Comment != "" {
		text += fmt.Sprintf(" (%s)", p.Comment)
	}

	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName))
	}

	return text, color, pageLink
}

func getIssueCommentPayloadInfo(p *api.IssueCommentPayload, linkFormatter linkFormatter, withSender bool) (string, string, int) {
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	issueTitle := fmt.Sprintf("#%d %s", p.Issue.Index, p.Issue.Title)

	var text, typ, titleLink string
	color := yellowColor

	if p.IsPull {
		typ = "pull request"
		titleLink = linkFormatter(p.Comment.PRURL, issueTitle)
	} else {
		typ = "issue"
		titleLink = linkFormatter(p.Comment.IssueURL, issueTitle)
	}

	switch p.Action {
	case api.HookIssueCommentCreated:
		text = fmt.Sprintf("[%s] New comment on %s %s", repoLink, typ, titleLink)
		if p.IsPull {
			color = greenColorLight
		} else {
			color = orangeColorLight
		}
	case api.HookIssueCommentEdited:
		text = fmt.Sprintf("[%s] Comment edited on %s %s", repoLink, typ, titleLink)
	case api.HookIssueCommentDeleted:
		text = fmt.Sprintf("[%s] Comment deleted on %s %s", repoLink, typ, titleLink)
		color = redColor
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName))
	}

	return text, issueTitle, color
}

func getPackagePayloadInfo(p *api.PackagePayload, linkFormatter linkFormatter, withSender bool) (text string, color int) {
	refLink := linkFormatter(p.Package.HTMLURL, p.Package.Name+":"+p.Package.Version)

	switch p.Action {
	case api.HookPackageCreated:
		text = fmt.Sprintf("Package created: %s", refLink)
		color = greenColor
	case api.HookPackageDeleted:
		text = fmt.Sprintf("Package deleted: %s", refLink)
		color = redColor
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName))
	}

	return text, color
}

func getStatusPayloadInfo(p *api.CommitStatusPayload, linkFormatter linkFormatter, withSender bool) (text string, color int) {
	refLink := linkFormatter(p.TargetURL, p.Context+"["+p.SHA+"]:"+p.Description)

	text = fmt.Sprintf("Commit Status changed: %s", refLink)
	color = greenColor
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+url.PathEscape(p.Sender.UserName), p.Sender.UserName))
	}

	return text, color
}

// ToHook convert models.Webhook to api.Hook
// This function is not part of the convert package to prevent an import cycle
func ToHook(repoLink string, w *webhook_model.Webhook) (*api.Hook, error) {
	config := map[string]string{
		"url":          w.URL,
		"content_type": w.ContentType.Name(),
	}
	if w.Type == webhook_module.SLACK {
		s := GetSlackHook(w)
		config["channel"] = s.Channel
		config["username"] = s.Username
		config["icon_url"] = s.IconURL
		config["color"] = s.Color
	}

	authorizationHeader, err := w.HeaderAuthorization()
	if err != nil {
		return nil, err
	}

	return &api.Hook{
		ID:                  w.ID,
		Type:                w.Type,
		URL:                 fmt.Sprintf("%s/settings/hooks/%d", repoLink, w.ID),
		Active:              w.IsActive,
		Config:              config,
		Events:              w.EventsArray(),
		AuthorizationHeader: authorizationHeader,
		Updated:             w.UpdatedUnix.AsTime(),
		Created:             w.CreatedUnix.AsTime(),
		BranchFilter:        w.BranchFilter,
	}, nil
}
