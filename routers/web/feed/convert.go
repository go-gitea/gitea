// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package feed

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"

	"github.com/gorilla/feeds"
)

func toBranchLink(act *models.Action) string {
	return act.GetRepoLink() + "/src/branch/" + util.PathEscapeSegments(act.GetBranch())
}

func toTagLink(act *models.Action) string {
	return act.GetRepoLink() + "/src/tag/" + util.PathEscapeSegments(act.GetTag())
}

func toIssueLink(act *models.Action) string {
	return act.GetRepoLink() + "/issues/" + url.PathEscape(act.GetIssueInfos()[0])
}

func toPullLink(act *models.Action) string {
	return act.GetRepoLink() + "/pulls/" + url.PathEscape(act.GetIssueInfos()[0])
}

func toSrcLink(act *models.Action) string {
	return act.GetRepoLink() + "/src/" + util.PathEscapeSegments(act.GetBranch())
}

func toReleaseLink(act *models.Action) string {
	return act.GetRepoLink() + "/releases/tag/" + util.PathEscapeSegments(act.GetBranch())
}

// feedActionsToFeedItems convert gitea's Action feed to feeds Item
func feedActionsToFeedItems(ctx *context.Context, actions []*models.Action) (items []*feeds.Item, err error) {
	for _, act := range actions {
		act.LoadActUser()

		content, desc, title := "", "", ""

		link := &feeds.Link{Href: act.GetCommentLink()}

		// title
		title = act.ActUser.DisplayName() + " "
		switch act.OpType {
		case models.ActionCreateRepo:
			title += ctx.TrHTMLEscapeArgs("action.create_repo", act.GetRepoLink(), act.ShortRepoPath())
			link.Href = act.GetRepoLink()
		case models.ActionRenameRepo:
			title += ctx.TrHTMLEscapeArgs("action.rename_repo", act.GetContent(), act.GetRepoLink(), act.ShortRepoPath())
			link.Href = act.GetRepoLink()
		case models.ActionCommitRepo:
			link.Href = toBranchLink(act)
			if len(act.Content) != 0 {
				title += ctx.TrHTMLEscapeArgs("action.commit_repo", act.GetRepoLink(), link.Href, act.GetBranch(), act.ShortRepoPath())
			} else {
				title += ctx.TrHTMLEscapeArgs("action.create_branch", act.GetRepoLink(), link.Href, act.GetBranch(), act.ShortRepoPath())
			}
		case models.ActionCreateIssue:
			link.Href = toIssueLink(act)
			title += ctx.TrHTMLEscapeArgs("action.create_issue", link.Href, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionCreatePullRequest:
			link.Href = toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.create_pull_request", link.Href, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionTransferRepo:
			link.Href = act.GetRepoLink()
			title += ctx.TrHTMLEscapeArgs("action.transfer_repo", act.GetContent(), act.GetRepoLink(), act.ShortRepoPath())
		case models.ActionPushTag:
			link.Href = toTagLink(act)
			title += ctx.TrHTMLEscapeArgs("action.push_tag", act.GetRepoLink(), link.Href, act.GetTag(), act.ShortRepoPath())
		case models.ActionCommentIssue:
			issueLink := toIssueLink(act)
			if link.Href == "#" {
				link.Href = issueLink
			}
			title += ctx.TrHTMLEscapeArgs("action.comment_issue", issueLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionMergePullRequest:
			pullLink := toPullLink(act)
			if link.Href == "#" {
				link.Href = pullLink
			}
			title += ctx.TrHTMLEscapeArgs("action.merge_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionCloseIssue:
			issueLink := toIssueLink(act)
			if link.Href == "#" {
				link.Href = issueLink
			}
			title += ctx.TrHTMLEscapeArgs("action.close_issue", issueLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionReopenIssue:
			issueLink := toIssueLink(act)
			if link.Href == "#" {
				link.Href = issueLink
			}
			title += ctx.TrHTMLEscapeArgs("action.reopen_issue", issueLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionClosePullRequest:
			pullLink := toPullLink(act)
			if link.Href == "#" {
				link.Href = pullLink
			}
			title += ctx.TrHTMLEscapeArgs("action.close_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionReopenPullRequest:
			pullLink := toPullLink(act)
			if link.Href == "#" {
				link.Href = pullLink
			}
			title += ctx.TrHTMLEscapeArgs("action.reopen_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionDeleteTag:
			link.Href = act.GetRepoLink()
			title += ctx.TrHTMLEscapeArgs("action.delete_tag", act.GetRepoLink(), act.GetTag(), act.ShortRepoPath())
		case models.ActionDeleteBranch:
			link.Href = act.GetRepoLink()
			title += ctx.TrHTMLEscapeArgs("action.delete_branch", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
		case models.ActionMirrorSyncPush:
			srcLink := toSrcLink(act)
			if link.Href == "#" {
				link.Href = srcLink
			}
			title += ctx.TrHTMLEscapeArgs("action.mirror_sync_push", act.GetRepoLink(), srcLink, act.GetBranch(), act.ShortRepoPath())
		case models.ActionMirrorSyncCreate:
			srcLink := toSrcLink(act)
			if link.Href == "#" {
				link.Href = srcLink
			}
			title += ctx.TrHTMLEscapeArgs("action.mirror_sync_create", act.GetRepoLink(), srcLink, act.GetBranch(), act.ShortRepoPath())
		case models.ActionMirrorSyncDelete:
			link.Href = act.GetRepoLink()
			title += ctx.TrHTMLEscapeArgs("action.mirror_sync_delete", act.GetRepoLink(), act.GetBranch(), act.ShortRepoPath())
		case models.ActionApprovePullRequest:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.approve_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionRejectPullRequest:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.reject_pull_request", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionCommentPull:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.comment_pull", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionPublishRelease:
			releaseLink := toReleaseLink(act)
			if link.Href == "#" {
				link.Href = releaseLink
			}
			title += ctx.TrHTMLEscapeArgs("action.publish_release", act.GetRepoLink(), releaseLink, act.ShortRepoPath(), act.Content)
		case models.ActionPullReviewDismissed:
			pullLink := toPullLink(act)
			title += ctx.TrHTMLEscapeArgs("action.review_dismissed", pullLink, act.GetIssueInfos()[0], act.ShortRepoPath(), act.GetIssueInfos()[1])
		case models.ActionStarRepo:
			link.Href = act.GetRepoLink()
			title += ctx.TrHTMLEscapeArgs("action.starred_repo", act.GetRepoLink(), act.GetRepoPath())
		case models.ActionWatchRepo:
			link.Href = act.GetRepoLink()
			title += ctx.TrHTMLEscapeArgs("action.watched_repo", act.GetRepoLink(), act.GetRepoPath())
		default:
			return nil, fmt.Errorf("unknown action type: %v", act.OpType)
		}

		// description & content
		{
			switch act.OpType {
			case models.ActionCommitRepo, models.ActionMirrorSyncPush:
				push := templates.ActionContent2Commits(act)
				repoLink := act.GetRepoLink()

				for _, commit := range push.Commits {
					if len(desc) != 0 {
						desc += "\n\n"
					}
					desc += fmt.Sprintf("<a href=\"%s\">%s</a>\n%s",
						html.EscapeString(fmt.Sprintf("%s/commit/%s", act.GetRepoLink(), commit.Sha1)),
						commit.Sha1,
						templates.RenderCommitMessage(commit.Message, repoLink, nil),
					)
				}

				if push.Len > 1 {
					link = &feeds.Link{Href: fmt.Sprintf("%s/%s", setting.AppSubURL, push.CompareURL)}
				} else if push.Len == 1 {
					link = &feeds.Link{Href: fmt.Sprintf("%s/commit/%s", act.GetRepoLink(), push.Commits[0].Sha1)}
				}

			case models.ActionCreateIssue, models.ActionCreatePullRequest:
				desc = strings.Join(act.GetIssueInfos(), "#")
				content = act.GetIssueContent()
			case models.ActionCommentIssue, models.ActionApprovePullRequest, models.ActionRejectPullRequest, models.ActionCommentPull:
				desc = act.GetIssueTitle()
				comment := act.GetIssueInfos()[1]
				if len(comment) != 0 {
					desc += "\n\n" + comment
				}
			case models.ActionMergePullRequest:
				desc = act.GetIssueInfos()[1]
			case models.ActionCloseIssue, models.ActionReopenIssue, models.ActionClosePullRequest, models.ActionReopenPullRequest:
				desc = act.GetIssueTitle()
			case models.ActionPullReviewDismissed:
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
			Id:      strconv.FormatInt(act.ID, 10),
			Created: act.CreatedUnix.AsTime(),
			Content: content,
		})
	}
	return
}
