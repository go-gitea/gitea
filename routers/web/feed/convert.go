// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package feed

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"

	"github.com/gorilla/feeds"
)

func toBranchLink(act *activities_model.Action) string {
	return act.GetRepoAbsoluteLink() + "/src/branch/" + util.PathEscapeSegments(act.GetBranch())
}

func toTagLink(act *activities_model.Action) string {
	return act.GetRepoAbsoluteLink() + "/src/tag/" + util.PathEscapeSegments(act.GetTag())
}

func toIssueLink(act *activities_model.Action) string {
	return act.GetRepoAbsoluteLink() + "/issues/" + url.PathEscape(act.GetIssueInfos()[0])
}

func toPullLink(act *activities_model.Action) string {
	return act.GetRepoAbsoluteLink() + "/pulls/" + url.PathEscape(act.GetIssueInfos()[0])
}

func toSrcLink(act *activities_model.Action) string {
	return act.GetRepoAbsoluteLink() + "/src/" + util.PathEscapeSegments(act.GetBranch())
}

func toReleaseLink(act *activities_model.Action) string {
	return act.GetRepoAbsoluteLink() + "/releases/tag/" + util.PathEscapeSegments(act.GetBranch())
}

// renderMarkdown creates a minimal markdown render context from an action.
// If rendering fails, the original markdown text is returned
func renderMarkdown(ctx *context.Context, act *activities_model.Action, content string) string {
	markdownCtx := &markup.RenderContext{
		Ctx:       ctx,
		URLPrefix: act.GetRepoLink(),
		Type:      markdown.MarkupName,
		Metas: map[string]string{
			"user": act.GetRepoUserName(),
			"repo": act.GetRepoName(),
		},
	}
	markdown, err := markdown.RenderString(markdownCtx, content)
	if err != nil {
		return content
	}
	return markdown
}

// feedActionsToFeedItems convert gitea's Action feed to feeds Item
func feedActionsToFeedItems(ctx *context.Context, actions activities_model.ActionList) (items []*feeds.Item, err error) {
	for _, act := range actions {
		act.LoadActUser()

		var content, desc, title string

		link := &feeds.Link{Href: act.GetCommentLink()}

		// title
		title = act.ActUser.DisplayName() + " "
		switch act.OpType {
		case activities_model.ActionCreateRepo:
			title += ctx.TrHTMLEscapeArgs("action.create_repo", act.GetRepoAbsoluteLink(), act.ShortRepoPath())
			link.Href = act.GetRepoAbsoluteLink()
		case activities_model.ActionRenameRepo:
			title += ctx.TrHTMLEscapeArgs("action.rename_repo", act.GetContent(), act.GetRepoAbsoluteLink(), act.ShortRepoPath())
			link.Href = act.GetRepoAbsoluteLink()
		case activities_model.ActionCommitRepo:
			link.Href = toBranchLink(act)
			if len(act.Content) != 0 {
				title += ctx.TrHTMLEscapeArgs("action.commit_repo", act.GetRepoAbsoluteLink(), link.Href, act.GetBranch(), act.ShortRepoPath())
			} else {
				title += ctx.TrHTMLEscapeArgs("action.create_branch", act.GetRepoAbsoluteLink(), link.Href, act.GetBranch(), act.ShortRepoPath())
			}
		case activities_model.ActionCreateIssue:
			link.Href = toIssueLink(act)
			title += ctx.TrHTMLEscapeArgs("action.create_issue", link.Href, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionCreatePullRequest:
			link.Href = toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.create_pull_request", link.Href, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionTransferRepo:
			link.Href = act.GetRepoAbsoluteLink()
			title += ctx.TrHTMLEscapeArgs("action.transfer_repo", act.GetContent(), act.GetRepoAbsoluteLink(), act.ShortRepoPath())
		case activities_model.ActionPushTag:
			link.Href = toTagLink(act)
			title += ctx.TrHTMLEscapeArgs("action.push_tag", act.GetRepoAbsoluteLink(), link.Href, act.GetTag(), act.ShortRepoPath())
		case activities_model.ActionCommentIssue:
			issueLink := toIssueLink(act)
			if link.Href == "#" {
				link.Href = issueLink
			}
			title += ctx.TrHTMLEscapeArgs("action.comment_issue", issueLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionMergePullRequest:
			pullLink := toPullLink(act)
			if link.Href == "#" {
				link.Href = pullLink
			}
			title += ctx.TrHTMLEscapeArgs("action.merge_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionAutoMergePullRequest:
			pullLink := toPullLink(act)
			if link.Href == "#" {
				link.Href = pullLink
			}
			title += ctx.TrHTMLEscapeArgs("action.auto_merge_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionCloseIssue:
			issueLink := toIssueLink(act)
			if link.Href == "#" {
				link.Href = issueLink
			}
			title += ctx.TrHTMLEscapeArgs("action.close_issue", issueLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionReopenIssue:
			issueLink := toIssueLink(act)
			if link.Href == "#" {
				link.Href = issueLink
			}
			title += ctx.TrHTMLEscapeArgs("action.reopen_issue", issueLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionClosePullRequest:
			pullLink := toPullLink(act)
			if link.Href == "#" {
				link.Href = pullLink
			}
			title += ctx.TrHTMLEscapeArgs("action.close_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionReopenPullRequest:
			pullLink := toPullLink(act)
			if link.Href == "#" {
				link.Href = pullLink
			}
			title += ctx.TrHTMLEscapeArgs("action.reopen_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionDeleteTag:
			link.Href = act.GetRepoAbsoluteLink()
			title += ctx.TrHTMLEscapeArgs("action.delete_tag", act.GetRepoAbsoluteLink(), act.GetTag(), act.ShortRepoPath())
		case activities_model.ActionDeleteBranch:
			link.Href = act.GetRepoAbsoluteLink()
			title += ctx.TrHTMLEscapeArgs("action.delete_branch", act.GetRepoAbsoluteLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
		case activities_model.ActionMirrorSyncPush:
			srcLink := toSrcLink(act)
			if link.Href == "#" {
				link.Href = srcLink
			}
			title += ctx.TrHTMLEscapeArgs("action.mirror_sync_push", act.GetRepoAbsoluteLink(), srcLink, act.GetBranch(), act.ShortRepoPath())
		case activities_model.ActionMirrorSyncCreate:
			srcLink := toSrcLink(act)
			if link.Href == "#" {
				link.Href = srcLink
			}
			title += ctx.TrHTMLEscapeArgs("action.mirror_sync_create", act.GetRepoAbsoluteLink(), srcLink, act.GetBranch(), act.ShortRepoPath())
		case activities_model.ActionMirrorSyncDelete:
			link.Href = act.GetRepoAbsoluteLink()
			title += ctx.TrHTMLEscapeArgs("action.mirror_sync_delete", act.GetRepoAbsoluteLink(), act.GetBranch(), act.ShortRepoPath())
		case activities_model.ActionApprovePullRequest:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.approve_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionRejectPullRequest:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.reject_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionCommentPull:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.comment_pull", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case activities_model.ActionPublishRelease:
			releaseLink := toReleaseLink(act)
			if link.Href == "#" {
				link.Href = releaseLink
			}
			title += ctx.TrHTMLEscapeArgs("action.publish_release", act.GetRepoAbsoluteLink(), releaseLink, act.ShortRepoPath(), act.Content)
		case activities_model.ActionPullReviewDismissed:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.review_dismissed", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath(), act.GetIssueInfos()[1])
		case activities_model.ActionStarRepo:
			link.Href = act.GetRepoAbsoluteLink()
			title += ctx.TrHTMLEscapeArgs("action.starred_repo", act.GetRepoAbsoluteLink(), act.GetRepoPath())
		case activities_model.ActionWatchRepo:
			link.Href = act.GetRepoAbsoluteLink()
			title += ctx.TrHTMLEscapeArgs("action.watched_repo", act.GetRepoAbsoluteLink(), act.GetRepoPath())
		default:
			return nil, fmt.Errorf("unknown action type: %v", act.OpType)
		}

		// description & content
		{
			switch act.OpType {
			case activities_model.ActionCommitRepo, activities_model.ActionMirrorSyncPush:
				push := templates.ActionContent2Commits(act)
				repoLink := act.GetRepoAbsoluteLink()

				for _, commit := range push.Commits {
					if len(desc) != 0 {
						desc += "\n\n"
					}
					desc += fmt.Sprintf("<a href=\"%s\">%s</a>\n%s",
						html.EscapeString(fmt.Sprintf("%s/commit/%s", act.GetRepoAbsoluteLink(), commit.Sha1)),
						commit.Sha1,
						templates.RenderCommitMessage(ctx, commit.Message, repoLink, nil),
					)
				}

				if push.Len > 1 {
					link = &feeds.Link{Href: fmt.Sprintf("%s/%s", setting.AppSubURL, push.CompareURL)}
				} else if push.Len == 1 {
					link = &feeds.Link{Href: fmt.Sprintf("%s/commit/%s", act.GetRepoAbsoluteLink(), push.Commits[0].Sha1)}
				}

			case activities_model.ActionCreateIssue, activities_model.ActionCreatePullRequest:
				desc = strings.Join(act.GetIssueInfos(), "#")
				content = renderMarkdown(ctx, act, act.GetIssueContent())
			case activities_model.ActionCommentIssue, activities_model.ActionApprovePullRequest, activities_model.ActionRejectPullRequest, activities_model.ActionCommentPull:
				desc = act.GetIssueTitle()
				comment := act.GetIssueInfos()[1]
				if len(comment) != 0 {
					desc += "\n\n" + renderMarkdown(ctx, act, comment)
				}
			case activities_model.ActionMergePullRequest, activities_model.ActionAutoMergePullRequest:
				desc = act.GetIssueInfos()[1]
			case activities_model.ActionCloseIssue, activities_model.ActionReopenIssue, activities_model.ActionClosePullRequest, activities_model.ActionReopenPullRequest:
				desc = act.GetIssueTitle()
			case activities_model.ActionPullReviewDismissed:
				desc = ctx.Tr("action.review_dismissed_reason") + "\n\n" + act.GetIssueInfos()[2]
			}
		}
		if len(content) == 0 {
			content = desc
		}

		items = append(items, &feeds.Item{
			Title:       title,
			Link:        link,
			Description: desc,
			Author: &feeds.Author{
				Name:  act.ActUser.DisplayName(),
				Email: act.ActUser.GetEmail(),
			},
			Id:      fmt.Sprintf("%v: %v", strconv.FormatInt(act.ID, 10), link.Href),
			Created: act.CreatedUnix.AsTime(),
			Content: content,
		})
	}
	return items, err
}

// GetFeedType return if it is a feed request and altered name and feed type.
func GetFeedType(name string, req *http.Request) (bool, string, string) {
	if strings.HasSuffix(name, ".rss") ||
		strings.Contains(req.Header.Get("Accept"), "application/rss+xml") {
		return true, strings.TrimSuffix(name, ".rss"), "rss"
	}

	if strings.HasSuffix(name, ".atom") ||
		strings.Contains(req.Header.Get("Accept"), "application/atom+xml") {
		return true, strings.TrimSuffix(name, ".atom"), "atom"
	}

	return false, name, ""
}
