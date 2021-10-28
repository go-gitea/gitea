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

	"github.com/gorilla/feeds"
)

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
			title += ctx.Tr("action.create_repo", act.GetRepoLink(), act.ShortRepoPath())
		case models.ActionRenameRepo:
			title += ctx.Tr("action.rename_repo", act.GetContent(), act.GetRepoLink(), act.ShortRepoPath())
		case models.ActionCommitRepo:
			branchLink := act.GetBranch()
			if len(act.Content) != 0 {
				title += ctx.Tr("action.commit_repo", act.GetRepoLink(), branchLink, act.GetBranch(), act.ShortRepoPath())
			} else {
				title += ctx.Tr("action.create_branch", act.GetRepoLink(), branchLink, act.GetBranch(), act.ShortRepoPath())
			}
		case models.ActionCreateIssue:
			title += ctx.Tr("action.create_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionCreatePullRequest:
			title += ctx.Tr("action.create_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionTransferRepo:
			title += ctx.Tr("action.transfer_repo", act.GetContent(), act.GetRepoLink(), act.ShortRepoPath())
		case models.ActionPushTag:
			title += ctx.Tr("action.push_tag", act.GetRepoLink(), url.QueryEscape(act.GetTag()), act.ShortRepoPath())
		case models.ActionCommentIssue:
			title += ctx.Tr("action.comment_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionMergePullRequest:
			title += ctx.Tr("action.merge_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionCloseIssue:
			title += ctx.Tr("action.close_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionReopenIssue:
			title += ctx.Tr("action.reopen_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionClosePullRequest:
			title += ctx.Tr("action.close_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionReopenPullRequest:
			title += ctx.Tr("action.reopen_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath)
		case models.ActionDeleteTag:
			title += ctx.Tr("action.delete_tag", act.GetRepoLink(), html.EscapeString(act.GetTag()), act.ShortRepoPath())
		case models.ActionDeleteBranch:
			title += ctx.Tr("action.delete_branch", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
		case models.ActionMirrorSyncPush:
			title += ctx.Tr("action.mirror_sync_push", act.GetRepoLink(), url.QueryEscape(act.GetBranch()), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
		case models.ActionMirrorSyncCreate:
			title += ctx.Tr("action.mirror_sync_create", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
		case models.ActionMirrorSyncDelete:
			title += ctx.Tr("action.mirror_sync_delete", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
		case models.ActionApprovePullRequest:
			title += ctx.Tr("action.approve_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionRejectPullRequest:
			title += ctx.Tr("action.reject_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionCommentPull:
			title += ctx.Tr("action.comment_pull", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
		case models.ActionPublishRelease:
			title += ctx.Tr("action.publish_release", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath(), act.Content)
		case models.ActionPullReviewDismissed:
			title += ctx.Tr("action.review_dismissed", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath(), act.GetIssueInfos()[1])
		case models.ActionStarRepo:
			title += ctx.Tr("action.starred_repo", act.GetRepoLink(), act.GetRepoPath())
			link = &feeds.Link{Href: act.GetRepoLink()}
		case models.ActionWatchRepo:
			title += ctx.Tr("action.watched_repo", act.GetRepoLink(), act.GetRepoPath())
			link = &feeds.Link{Href: act.GetRepoLink()}
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
						fmt.Sprintf("%s/commit/%s", act.GetRepoLink(), commit.Sha1),
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
